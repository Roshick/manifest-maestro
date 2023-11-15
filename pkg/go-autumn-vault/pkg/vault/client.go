package vault

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	aulogging "github.com/StephanHCB/go-autumn-logging"
	aurestclientprometheus "github.com/StephanHCB/go-autumn-restclient-prometheus"
	aurestclientapi "github.com/StephanHCB/go-autumn-restclient/api"
	auresthttpclient "github.com/StephanHCB/go-autumn-restclient/implementation/httpclient"
	aurestlogging "github.com/StephanHCB/go-autumn-restclient/implementation/requestlogging"
	"github.com/go-http-utils/headers"
)

type ClientImpl struct {
	aurestclientapi.Client
	config    Config
	authToken *string
}

type K8sAuthRequest struct {
	Jwt  string `json:"jwt"`
	Role string `json:"role"`
}

type K8sAuthResponse struct {
	Auth   *K8sAuth `json:"auth"`
	Errors []string `json:"httperrors"`
}

type K8sAuth struct {
	ClientToken string `json:"client_token"`
}

type SecretsResponse struct {
	Data   *SecretsResponseData `json:"data"`
	Errors []string             `json:"httperrors"`
}

type SecretsResponseData struct {
	Data map[string]string `json:"data"`
}

func NewClient(ctx context.Context, config Config) (*ClientImpl, error) {
	authToken := new(string)
	client, err := auresthttpclient.New(15*time.Second, config.PublicCertificate(), requestManipulator(authToken))
	if err != nil {
		return nil, err
	}
	aurestclientprometheus.InstrumentHttpClient(client)
	client = aurestlogging.New(client)
	vaultClient := &ClientImpl{
		Client:    client,
		config:    config,
		authToken: authToken,
	}
	if err = vaultClient.refreshAuthToken(ctx); err != nil {
		return nil, err
	}
	return vaultClient, nil
}

func requestManipulator(authToken *string) func(ctx context.Context, r *http.Request) {
	return func(ctx context.Context, r *http.Request) {
		r.Header.Set(headers.Accept, aurestclientapi.ContentTypeApplicationJson)
		if authToken != nil && *authToken != "" {
			r.Header.Set("X-Vault-Token", *authToken)
		}
	}
}

func (c *ClientImpl) refreshAuthToken(ctx context.Context) error {
	if c.config.AuthToken() != "" {
		authToken := c.config.AuthToken()
		*c.authToken = authToken
		aulogging.Logger.Ctx(ctx).Info().Print("using passed in vault token, skipping authentication with vault")
		return nil
	} else {
		aulogging.Logger.Ctx(ctx).Info().Print("authenticating with vault")

		remoteUrl := fmt.Sprintf("%s/v1/auth/%s/login", c.config.Server(), c.config.AuthKubernetesBackend())

		k8sToken, err := os.ReadFile(c.config.AuthKubernetesTokenPath())
		if err != nil {
			return fmt.Errorf("unable to read vault token file from path %s: %s", c.config.AuthKubernetesTokenPath(), err.Error())
		}

		requestDto := &K8sAuthRequest{
			Jwt:  string(k8sToken),
			Role: c.config.AuthKubernetesRole(),
		}

		responseDto := &K8sAuthResponse{}
		response := &aurestclientapi.ParsedResponse{
			Body: responseDto,
		}

		err = c.Perform(ctx, http.MethodPost, remoteUrl, requestDto, response)
		if err != nil {
			return err
		}

		if response.Status != http.StatusOK {
			return fmt.Errorf("did not receive http 200 from vault")
		}

		if len(responseDto.Errors) > 0 {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to authenticate with vault: %v", responseDto.Errors)
			return fmt.Errorf("got an httperrors array from vault")
		}
		if responseDto.Auth == nil || responseDto.Auth.ClientToken == "" {
			return fmt.Errorf("response from vault did not include a client_token")
		}

		authToken := responseDto.Auth.ClientToken
		*c.authToken = authToken
		return nil
	}
}

func (c *ClientImpl) ObtainSecrets(ctx context.Context, fullSecretsPath string) (map[string]string, error) {
	emptyMap := make(map[string]string)

	aulogging.Logger.Ctx(ctx).Info().Printf("querying vault for secrets, secret path %s", fullSecretsPath)

	remoteUrl := fmt.Sprintf("%s/v1/%s", c.config.Server(), fullSecretsPath)

	responseDto := &SecretsResponse{}
	response := &aurestclientapi.ParsedResponse{
		Body: responseDto,
	}

	err := c.Perform(ctx, http.MethodGet, remoteUrl, nil, response)
	if err != nil {
		return emptyMap, err
	}

	if response.Status != http.StatusOK {
		return emptyMap, fmt.Errorf("did not receive http 200 from vault")
	}

	if len(responseDto.Errors) > 0 {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to obtain secrets from vault: %v", responseDto.Errors)
		return emptyMap, fmt.Errorf("got an httperrors array from vault")
	}

	if responseDto.Data == nil {
		return emptyMap, fmt.Errorf("got no top level data structure from vault")
	}
	if responseDto.Data.Data == nil {
		return emptyMap, fmt.Errorf("got no second level data structure from vault")
	}

	return responseDto.Data.Data, nil
}
