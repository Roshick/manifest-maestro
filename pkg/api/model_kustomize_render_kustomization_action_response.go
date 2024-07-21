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
)

// checks if the KustomizeRenderKustomizationActionResponse type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &KustomizeRenderKustomizationActionResponse{}

// KustomizeRenderKustomizationActionResponse struct for KustomizeRenderKustomizationActionResponse
type KustomizeRenderKustomizationActionResponse struct {
	Manifests []Manifest `json:"manifests,omitempty"`
}

// NewKustomizeRenderKustomizationActionResponse instantiates a new KustomizeRenderKustomizationActionResponse object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewKustomizeRenderKustomizationActionResponse() *KustomizeRenderKustomizationActionResponse {
	this := KustomizeRenderKustomizationActionResponse{}
	return &this
}

// NewKustomizeRenderKustomizationActionResponseWithDefaults instantiates a new KustomizeRenderKustomizationActionResponse object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewKustomizeRenderKustomizationActionResponseWithDefaults() *KustomizeRenderKustomizationActionResponse {
	this := KustomizeRenderKustomizationActionResponse{}
	return &this
}

// GetManifests returns the Manifests field value if set, zero value otherwise.
func (o *KustomizeRenderKustomizationActionResponse) GetManifests() []Manifest {
	if o == nil || IsNil(o.Manifests) {
		var ret []Manifest
		return ret
	}
	return o.Manifests
}

// GetManifestsOk returns a tuple with the Manifests field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *KustomizeRenderKustomizationActionResponse) GetManifestsOk() ([]Manifest, bool) {
	if o == nil || IsNil(o.Manifests) {
		return nil, false
	}
	return o.Manifests, true
}

// HasManifests returns a boolean if a field has been set.
func (o *KustomizeRenderKustomizationActionResponse) HasManifests() bool {
	if o != nil && !IsNil(o.Manifests) {
		return true
	}

	return false
}

// SetManifests gets a reference to the given []Manifest and assigns it to the Manifests field.
func (o *KustomizeRenderKustomizationActionResponse) SetManifests(v []Manifest) {
	o.Manifests = v
}

func (o KustomizeRenderKustomizationActionResponse) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o KustomizeRenderKustomizationActionResponse) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Manifests) {
		toSerialize["manifests"] = o.Manifests
	}
	return toSerialize, nil
}

type NullableKustomizeRenderKustomizationActionResponse struct {
	value *KustomizeRenderKustomizationActionResponse
	isSet bool
}

func (v NullableKustomizeRenderKustomizationActionResponse) Get() *KustomizeRenderKustomizationActionResponse {
	return v.value
}

func (v *NullableKustomizeRenderKustomizationActionResponse) Set(val *KustomizeRenderKustomizationActionResponse) {
	v.value = val
	v.isSet = true
}

func (v NullableKustomizeRenderKustomizationActionResponse) IsSet() bool {
	return v.isSet
}

func (v *NullableKustomizeRenderKustomizationActionResponse) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableKustomizeRenderKustomizationActionResponse(val *KustomizeRenderKustomizationActionResponse) *NullableKustomizeRenderKustomizationActionResponse {
	return &NullableKustomizeRenderKustomizationActionResponse{value: val, isSet: true}
}

func (v NullableKustomizeRenderKustomizationActionResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableKustomizeRenderKustomizationActionResponse) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


