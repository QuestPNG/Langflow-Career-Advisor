package main

import "time"


type LangflowChatResponse struct {
	Outputs   []struct {
		Inputs struct {
			InputValue string `json:"input_value"`
		} `json:"inputs"`
		Outputs []struct {
			Results struct {
				Message struct {
					Text       string    `json:"text"`
					Sender     string    `json:"sender"`
					SenderName string    `json:"sender_name"`
					SessionID  string    `json:"session_id"`
					Timestamp  time.Time `json:"timestamp"`
					FlowID     string    `json:"flow_id"`
					Properties struct {
						Source struct {
							ID          string `json:"id"`
							DisplayName string `json:"display_name"`
							Source      string `json:"source"`
						} `json:"source"`
						Icon string `json:"icon"`
					} `json:"properties"`
					ComponentID string `json:"component_id"`
				} `json:"message"`
			} `json:"results"`
		} `json:"outputs"`
	} `json:"outputs"`
}
