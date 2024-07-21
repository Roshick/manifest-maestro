/*
Manifest Maestro

Renders Kubernetes manifests with the help of various tools such as Helm and Kustomize.

API version: v1.2.0
Contact: e.rieb@posteo.de
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package api

import (
	"encoding/json"
	"bytes"
	"fmt"
)

// checks if the HelmChartRepositoryChartReference type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &HelmChartRepositoryChartReference{}

// HelmChartRepositoryChartReference struct for HelmChartRepositoryChartReference
type HelmChartRepositoryChartReference struct {
	RepositoryURL string `json:"repositoryURL"`
	ChartName string `json:"chartName"`
	ChartVersion *string `json:"chartVersion,omitempty"`
}

type _HelmChartRepositoryChartReference HelmChartRepositoryChartReference

// NewHelmChartRepositoryChartReference instantiates a new HelmChartRepositoryChartReference object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewHelmChartRepositoryChartReference(repositoryURL string, chartName string) *HelmChartRepositoryChartReference {
	this := HelmChartRepositoryChartReference{}
	this.RepositoryURL = repositoryURL
	this.ChartName = chartName
	return &this
}

// NewHelmChartRepositoryChartReferenceWithDefaults instantiates a new HelmChartRepositoryChartReference object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewHelmChartRepositoryChartReferenceWithDefaults() *HelmChartRepositoryChartReference {
	this := HelmChartRepositoryChartReference{}
	return &this
}

// GetRepositoryURL returns the RepositoryURL field value
func (o *HelmChartRepositoryChartReference) GetRepositoryURL() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.RepositoryURL
}

// GetRepositoryURLOk returns a tuple with the RepositoryURL field value
// and a boolean to check if the value has been set.
func (o *HelmChartRepositoryChartReference) GetRepositoryURLOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.RepositoryURL, true
}

// SetRepositoryURL sets field value
func (o *HelmChartRepositoryChartReference) SetRepositoryURL(v string) {
	o.RepositoryURL = v
}

// GetChartName returns the ChartName field value
func (o *HelmChartRepositoryChartReference) GetChartName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.ChartName
}

// GetChartNameOk returns a tuple with the ChartName field value
// and a boolean to check if the value has been set.
func (o *HelmChartRepositoryChartReference) GetChartNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.ChartName, true
}

// SetChartName sets field value
func (o *HelmChartRepositoryChartReference) SetChartName(v string) {
	o.ChartName = v
}

// GetChartVersion returns the ChartVersion field value if set, zero value otherwise.
func (o *HelmChartRepositoryChartReference) GetChartVersion() string {
	if o == nil || IsNil(o.ChartVersion) {
		var ret string
		return ret
	}
	return *o.ChartVersion
}

// GetChartVersionOk returns a tuple with the ChartVersion field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *HelmChartRepositoryChartReference) GetChartVersionOk() (*string, bool) {
	if o == nil || IsNil(o.ChartVersion) {
		return nil, false
	}
	return o.ChartVersion, true
}

// HasChartVersion returns a boolean if a field has been set.
func (o *HelmChartRepositoryChartReference) HasChartVersion() bool {
	if o != nil && !IsNil(o.ChartVersion) {
		return true
	}

	return false
}

// SetChartVersion gets a reference to the given string and assigns it to the ChartVersion field.
func (o *HelmChartRepositoryChartReference) SetChartVersion(v string) {
	o.ChartVersion = &v
}

func (o HelmChartRepositoryChartReference) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o HelmChartRepositoryChartReference) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["repositoryURL"] = o.RepositoryURL
	toSerialize["chartName"] = o.ChartName
	if !IsNil(o.ChartVersion) {
		toSerialize["chartVersion"] = o.ChartVersion
	}
	return toSerialize, nil
}

func (o *HelmChartRepositoryChartReference) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"repositoryURL",
		"chartName",
	}

	allProperties := make(map[string]interface{})

	err = json.Unmarshal(data, &allProperties)

	if err != nil {
		return err;
	}

	for _, requiredProperty := range(requiredProperties) {
		if _, exists := allProperties[requiredProperty]; !exists {
			return fmt.Errorf("no value given for required property %v", requiredProperty)
		}
	}

	varHelmChartRepositoryChartReference := _HelmChartRepositoryChartReference{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varHelmChartRepositoryChartReference)

	if err != nil {
		return err
	}

	*o = HelmChartRepositoryChartReference(varHelmChartRepositoryChartReference)

	return err
}

type NullableHelmChartRepositoryChartReference struct {
	value *HelmChartRepositoryChartReference
	isSet bool
}

func (v NullableHelmChartRepositoryChartReference) Get() *HelmChartRepositoryChartReference {
	return v.value
}

func (v *NullableHelmChartRepositoryChartReference) Set(val *HelmChartRepositoryChartReference) {
	v.value = val
	v.isSet = true
}

func (v NullableHelmChartRepositoryChartReference) IsSet() bool {
	return v.isSet
}

func (v *NullableHelmChartRepositoryChartReference) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableHelmChartRepositoryChartReference(val *HelmChartRepositoryChartReference) *NullableHelmChartRepositoryChartReference {
	return &NullableHelmChartRepositoryChartReference{value: val, isSet: true}
}

func (v NullableHelmChartRepositoryChartReference) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableHelmChartRepositoryChartReference) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

