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
	"gopkg.in/validator.v2"
	"fmt"
)

// KustomizationReference - struct for KustomizationReference
type KustomizationReference struct {
	GitRepositoryPathReference *GitRepositoryPathReference
}

// GitRepositoryPathReferenceAsKustomizationReference is a convenience function that returns GitRepositoryPathReference wrapped in KustomizationReference
func GitRepositoryPathReferenceAsKustomizationReference(v *GitRepositoryPathReference) KustomizationReference {
	return KustomizationReference{
		GitRepositoryPathReference: v,
	}
}


// Unmarshal JSON data into one of the pointers in the struct
func (dst *KustomizationReference) UnmarshalJSON(data []byte) error {
	var err error
	match := 0
	// try to unmarshal data into GitRepositoryPathReference
	err = newStrictDecoder(data).Decode(&dst.GitRepositoryPathReference)
	if err == nil {
		jsonGitRepositoryPathReference, _ := json.Marshal(dst.GitRepositoryPathReference)
		if string(jsonGitRepositoryPathReference) == "{}" { // empty struct
			dst.GitRepositoryPathReference = nil
		} else {
			if err = validator.Validate(dst.GitRepositoryPathReference); err != nil {
				dst.GitRepositoryPathReference = nil
			} else {
				match++
			}
		}
	} else {
		dst.GitRepositoryPathReference = nil
	}

	if match > 1 { // more than 1 match
		// reset to nil
		dst.GitRepositoryPathReference = nil

		return fmt.Errorf("data matches more than one schema in oneOf(KustomizationReference)")
	} else if match == 1 {
		return nil // exactly one match
	} else { // no match
		return fmt.Errorf("data failed to match schemas in oneOf(KustomizationReference)")
	}
}

// Marshal data from the first non-nil pointers in the struct to JSON
func (src KustomizationReference) MarshalJSON() ([]byte, error) {
	if src.GitRepositoryPathReference != nil {
		return json.Marshal(&src.GitRepositoryPathReference)
	}

	return nil, nil // no data in oneOf schemas
}

// Get the actual instance
func (obj *KustomizationReference) GetActualInstance() (interface{}) {
	if obj == nil {
		return nil
	}
	if obj.GitRepositoryPathReference != nil {
		return obj.GitRepositoryPathReference
	}

	// all schemas are nil
	return nil
}

type NullableKustomizationReference struct {
	value *KustomizationReference
	isSet bool
}

func (v NullableKustomizationReference) Get() *KustomizationReference {
	return v.value
}

func (v *NullableKustomizationReference) Set(val *KustomizationReference) {
	v.value = val
	v.isSet = true
}

func (v NullableKustomizationReference) IsSet() bool {
	return v.isSet
}

func (v *NullableKustomizationReference) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableKustomizationReference(val *KustomizationReference) *NullableKustomizationReference {
	return &NullableKustomizationReference{value: val, isSet: true}
}

func (v NullableKustomizationReference) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableKustomizationReference) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

