// Package jsonutil contains various utilities for dealing with json data.
package jsonutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExampleMarshal() {
	type ColorGroup struct {
		ID     int
		Name   string
		Colors []string
	}

	group := ColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}

	b, err := Marshal(group)
	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(b)
	// Output:
	// {"ID":1,"Name":"Reds","Colors":["Crimson","Red","Ruby","Maroon"]}
}

func ExampleRemarshal() {
	type ColorGroup struct {
		ID     int
		Name   string
		Colors []string
	}

	group := ColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}

	var newGroup ColorGroup

	err := Remarshal(group, &newGroup)
	if err != nil {
		fmt.Println("error:", err)
	}

	out, err := Marshal(newGroup)
	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(out)
	// Output:
	// {"ID":1,"Name":"Reds","Colors":["Crimson","Red","Ruby","Maroon"]}
}

func ExampleIndent() {
	type Road struct {
		Name   string
		Number int
	}

	roads := []Road{
		{Name: "Diamond Fork", Number: 29},
		{Name: "Sheep Creek", Number: 51},
	}

	b, err := Marshal(roads)
	if err != nil {
		log.Fatal(err)
	}

	out, err := Indent(b)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(out)
	// Output:
	// [
	//   {
	//     "Name": "Diamond Fork",
	//     "Number": 29
	//   },
	//   {
	//     "Name": "Sheep Creek",
	//     "Number": 51
	//   }
	// ]
}

func TestIndent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
	}{
		{"Basic", "[{\"Name\":\"Diamond Fork\", \"Number\":29}, {\"Name\":\"Sheep Creek\", \"Number\":51}]"},
		{"BasicMoreWhitespace", "[\n{\"Name\":\"Diamond Fork\",     \"Number\":29}, {    \"Name\"   :   \"Sheep Creek\",    \"Number\":51}]"},
	}
	for _, tc := range testCases {
		out, err := Indent(tc.input)
		if err != nil {
			t.Errorf("error indenting JSON: %v", err)

			continue
		}

		correct, err := os.ReadFile(filepath.Join("testdata", t.Name()+tc.name+".golden"))
		if err != nil {
			t.Errorf("error reading file: %v", err)
		}

		assert.Equal(t, string(correct), out)
	}
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	group := struct {
		ID     int
		Name   string
		Colors []string
	}{
		1,
		"Reds",
		[]string{"Crimson", "Red", "Ruby", "Maroon"},
	}

	out, err := Marshal(group)
	if err != nil {
		t.Errorf("error in %s: %v", t.Name(), err)
	}

	correct, err := os.ReadFile(filepath.Join("testdata", t.Name()+".golden"))
	if err != nil {
		t.Errorf("error reading golden file in %s: %v", t.Name(), err)
	}

	assert.Equal(t, string(correct), out)
}

func TestUnmarshalFile(t *testing.T) {
	t.Parallel()

	var contents any

	// missing file
	err1 := UnmarshalFile(filepath.Join("testdata", "TestUnmarshalFileMissing.json"), &contents)
	require.Error(t, err1, "expected readfile error")

	// non json content
	err2 := UnmarshalFile(filepath.Join("testdata", "TestUnmarshalFileParseError.json"), &contents)
	require.Error(t, err2, "expected json parsing error")

	// valid json content
	err3 := UnmarshalFile(filepath.Join("testdata", "TestUnmarshalFileValid.json"), &contents)
	require.NoError(t, err3, "message should parse successfully")
}

func TestRemarshal(t *testing.T) {
	t.Parallel()

	prop := make(map[string]string)
	prop["RunCommand"] = "echo"
	prop2 := make(map[string]string)
	prop2["command"] = "echo"

	type Property struct {
		RunCommand string
	}

	var newProp Property

	var newProp2 Property

	err := Remarshal(prop, &newProp)
	require.NoError(t, err, "message should remarshal successfully")
	err = Remarshal(prop2, &newProp2)
	require.NoError(t, err, "key mismatch should not report error")
	assert.Equal(t, Property{}, newProp2, "mismatched remarshal should return an empty object")
}

func TestRemarshalInvalidInput(t *testing.T) {
	t.Parallel()

	// Using channel as unsupported json type
	// Expect an error and no change to input object
	badInput := make(chan bool)

	type Output struct {
		name string //nolint:unused
	}

	var output Output
	// Save an copy of output to compare to after Remarshal has been called to confirm no changes were made
	originalOutput := output
	err := Remarshal(badInput, &output)
	require.Error(t, err)

	if !assert.ObjectsAreEqual(originalOutput, output) {
		t.Fatalf("Object was modified by call to Remarshal")
	}
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	content := `{"parameter": "1"}`

	type TestStruct struct {
		Parameter string `json:"parameter"`
	}

	output := TestStruct{}
	err := Unmarshal(content, &output)
	require.NoError(t, err, "Message should parse correctly")
	assert.Equal(t, "1", output.Parameter)
}

func TestUnmarshalExtraInput(t *testing.T) {
	t.Parallel()

	content := `{"parameter": "1", "name": "Richard"}`

	type TestStruct struct {
		Parameter string `json:"parameter"`
	}

	output := TestStruct{}
	err := Unmarshal(content, &output)
	require.NoError(t, err, "Message should parse correctly")
	assert.Equal(t, "1", output.Parameter)
}

func TestUnmarshalInvalidInput(t *testing.T) {
	t.Parallel()

	content := "Hello"

	var dest any
	err := Unmarshal(content, &dest)
	require.Error(t, err, "This is not json format. Error expected")
}

func TestMarshalIndent(t *testing.T) {
	t.Parallel()

	group := struct {
		ID     int
		Name   string
		Colors []string
	}{
		1,
		"Reds",
		[]string{"Crimson", "Red", "Ruby", "Maroon"},
	}

	correct, err := os.ReadFile(filepath.Join("testdata", t.Name()+".golden"))
	if err != nil {
		t.Errorf("error: %v", err)
		t.FailNow()
	}

	out, err := MarshalIndent(group)
	if err != nil {
		t.Errorf("error: %v", err)
		t.FailNow()
	}

	assert.Equal(t, string(correct), out)
}

func TestMarshalIndentErrorsOnInvalidInput(t *testing.T) {
	t.Parallel()

	// Using channel as invalid input
	// Breaks the same for any json-invalid types
	_, err := MarshalIndent(make(chan int))
	require.Error(t, err)
}
