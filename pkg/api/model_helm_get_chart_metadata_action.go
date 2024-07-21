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

// checks if the HelmGetChartMetadataAction type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &HelmGetChartMetadataAction{}

// HelmGetChartMetadataAction struct for HelmGetChartMetadataAction
type HelmGetChartMetadataAction struct {
	Reference HelmChartReference `json:"reference"`
}

type _HelmGetChartMetadataAction HelmGetChartMetadataAction

// NewHelmGetChartMetadataAction instantiates a new HelmGetChartMetadataAction object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewHelmGetChartMetadataAction(reference HelmChartReference) *HelmGetChartMetadataAction {
	this := HelmGetChartMetadataAction{}
	this.Reference = reference
	return &this
}

// NewHelmGetChartMetadataActionWithDefaults instantiates a new HelmGetChartMetadataAction object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewHelmGetChartMetadataActionWithDefaults() *HelmGetChartMetadataAction {
	this := HelmGetChartMetadataAction{}
	return &this
}

// GetReference returns the Reference field value
func (o *HelmGetChartMetadataAction) GetReference() HelmChartReference {
	if o == nil {
		var ret HelmChartReference
		return ret
	}

	return o.Reference
}

// GetReferenceOk returns a tuple with the Reference field value
// and a boolean to check if the value has been set.
func (o *HelmGetChartMetadataAction) GetReferenceOk() (*HelmChartReference, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Reference, true
}

// SetReference sets field value
func (o *HelmGetChartMetadataAction) SetReference(v HelmChartReference) {
	o.Reference = v
}

func (o HelmGetChartMetadataAction) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o HelmGetChartMetadataAction) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["reference"] = o.Reference
	return toSerialize, nil
}

func (o *HelmGetChartMetadataAction) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"reference",
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

	varHelmGetChartMetadataAction := _HelmGetChartMetadataAction{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varHelmGetChartMetadataAction)

	if err != nil {
		return err
	}

	*o = HelmGetChartMetadataAction(varHelmGetChartMetadataAction)

	return err
}

type NullableHelmGetChartMetadataAction struct {
	value *HelmGetChartMetadataAction
	isSet bool
}

func (v NullableHelmGetChartMetadataAction) Get() *HelmGetChartMetadataAction {
	return v.value
}

func (v *NullableHelmGetChartMetadataAction) Set(val *HelmGetChartMetadataAction) {
	v.value = val
	v.isSet = true
}

func (v NullableHelmGetChartMetadataAction) IsSet() bool {
	return v.isSet
}

func (v *NullableHelmGetChartMetadataAction) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableHelmGetChartMetadataAction(val *HelmGetChartMetadataAction) *NullableHelmGetChartMetadataAction {
	return &NullableHelmGetChartMetadataAction{value: val, isSet: true}
}

func (v NullableHelmGetChartMetadataAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableHelmGetChartMetadataAction) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

