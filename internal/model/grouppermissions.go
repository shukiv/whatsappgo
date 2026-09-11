package model

import "errors"

// One permission per request: WhatsApp does not provide an atomic batch setter.
// Pointers distinguish an explicit false from an omitted value.
type GroupPermissionEdit struct {
	Field    string `json:"field"`
	Value    *bool  `json:"value"`
	Previous *bool  `json:"previous"`
}

func (e GroupPermissionEdit) Validate() error {
	if e.Value == nil || e.Previous == nil {
		return errors.New("value and previous group permission are required; reopen the setting")
	}
	switch e.Field {
	case "send_messages", "edit_info", "add_members", "approve_new_members":
		return nil
	default:
		return errors.New("unsupported group permission")
	}
}
