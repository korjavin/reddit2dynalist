package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	dynalistAPIURL = "https://dynalist.io/api/v1/inbox/add"
)

// DynalistRequest represents the request body for the Dynalist API
type DynalistRequest struct {
	Token    string `json:"token"`
	Index    int    `json:"index,omitempty"`
	Content  string `json:"content"`
	Note     string `json:"note,omitempty"`
	Checked  bool   `json:"checked,omitempty"`
	Checkbox bool   `json:"checkbox,omitempty"`
}

// DynalistResponse represents the response from the Dynalist API
type DynalistResponse struct {
	Code    string `json:"_code"`
	Message string `json:"_msg,omitempty"`
	FileID  string `json:"file_id,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	Index   int    `json:"index,omitempty"`
}

// AddToDynalist sends a message to the Dynalist inbox
func AddToDynalist(token, content string, note string) error {
	// Create request body
	reqBody := DynalistRequest{
		Token:   token,
		Content: content,
		Note:    note,
	}

	// Marshal request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", dynalistAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var dynalistResp DynalistResponse
	if err := json.NewDecoder(resp.Body).Decode(&dynalistResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check response code
	if dynalistResp.Code != "Ok" {
		if dynalistResp.Message != "" {
			return fmt.Errorf("dynalist API error: %s", dynalistResp.Message)
		}
		return fmt.Errorf("dynalist API error: code %s", dynalistResp.Code)
	}

	return nil
}
