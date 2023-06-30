package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

// IndentedJSON converts v to JSON string with 4-space indent
func IndentedJSON(v interface{}) string {
	byt, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "<Invalid JSON>"
	}
	return string(byt)
}

// CompactJSON converts v to compact JSON string
func CompactJSON(v interface{}) string {
	byt, err := json.Marshal(v)
	if err != nil {
		return "<Invalid JSON>"
	}
	return string(byt)
}

// WriteJSONToFile marshals v and write to file
func WriteJSONToFile(v interface{}, file string) error {
	fd, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer func() {
		_ = fd.Close()
	}()

	byt, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling json: %v", err)
	}
	_, err = fd.Write(byt)
	if err != nil {
		return fmt.Errorf("error writing content: %v", err)
	}
	return nil
}

// ReadJSONFromFile reads file content as JSON format, and unmarshalls to pointer v
// NOTE: v must be a pointer
func ReadJSONFromFile(file string, v interface{}) error {
	byt, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	err = json.Unmarshal(byt, v)
	if err != nil {
		return fmt.Errorf("error unmarshaling json: %v", err)
	}
	return nil
}

// ReadJSONResponse reads HTTP response body, unmarshalls the body to pointer v and returns the JSON bytes array
// NOTE: v must be a pointer
func ReadJSONResponse(resp *http.Response, v interface{}) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}

	byt, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("faild to read response body: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	err = json.Unmarshal(byt, v)
	if err != nil {
		return byt, fmt.Errorf("failed to unmarshal bytes to value: %v", err)
	}
	return byt, nil
}

// UnmarshalRawMessage unmarshalls JSON RawMessage to pointer v
// NOTE: v must be a pointer
func UnmarshalRawMessage(raw json.RawMessage, v interface{}) error {
	byt, err := raw.MarshalJSON()
	if err != nil {
		return err
	}
	err = json.Unmarshal(byt, v)
	if err != nil {
		return err
	}
	return nil
}
