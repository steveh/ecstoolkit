// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package jsonutil contains various utilities for dealing with json data.
package jsonutil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// jsonFormat json formatIndent.
const jsonFormat = "  "

// Indent indents a json string.
func Indent(jsonStr string) (string, error) {
	var dst bytes.Buffer

	if err := json.Indent(&dst, []byte(jsonStr), "", jsonFormat); err != nil {
		return "", fmt.Errorf("indenting JSON: %w", err)
	}

	return dst.String(), nil
}

// Remarshal marshals an object to Json then parses it back to another object.
// This is useful for example when we want to go from map[string]any
// to a more specific struct type or if we want a deep copy of the object.
func Remarshal(obj any, remarshalledObj any) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshaling object: %w", err)
	}

	err = json.Unmarshal(b, remarshalledObj)
	if err != nil {
		return fmt.Errorf("unmarshaling object: %w", err)
	}

	return nil
}

// Marshal marshals an object to a json string.
// Returns empty string if marshal fails.
func Marshal(obj any) (string, error) {
	resultB, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshaling object: %w", err)
	}

	return string(resultB), nil
}

// UnmarshalFile reads the content of a file then Unmarshals the content to an object.
func UnmarshalFile(filePath string, dest any) error {
	content, err := ioUtil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", filePath, err)
	}

	err = json.Unmarshal(content, dest)
	if err != nil {
		return fmt.Errorf("unmarshaling file content: %w", err)
	}

	return nil
}

// Unmarshal unmarshals the content in string format to an object.
func Unmarshal(jsonContent string, dest any) error {
	content := []byte(jsonContent)

	err := json.Unmarshal(content, dest)
	if err != nil {
		return fmt.Errorf("unmarshaling JSON content: %w", err)
	}

	return nil
}

// MarshalIndent is like Marshal but applies Indent to format the output.
// Returns empty string if marshal fails.
func MarshalIndent(obj any) (string, error) {
	// Make sure the output file keeps formal json format
	resultsByte, err := json.MarshalIndent(obj, "", jsonFormat)
	if err != nil {
		return "", fmt.Errorf("marshaling object with indent: %w", err)
	}

	return string(resultsByte), nil
}
