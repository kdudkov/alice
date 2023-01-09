package main

import (
	"strings"
)

type Capability struct {
	Type  string `json:"type"`
	State *State `json:"state,omitempty"`
}

type State struct {
	Instance     string        `json:"instance"`
	Value        any           `json:"value"`
	ActionResult *ActionResult `json:"action_result,omitempty"`
}

type ActionResult struct {
	Status       string `json:"status"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type Device struct {
	Id           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Room         string         `json:"room"`
	Type         string         `json:"type"`
	CustomData   any            `json:"custom_data"`
	Capabilities []*Capability  `json:"capabilities"`
	Properties   map[string]any `json:"properties,omitempty"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

type DevReq struct {
	Id           string        `json:"id"`
	CustomData   any           `json:"custom_data"`
	Capabilities []*Capability `json:"capabilities,omitempty"`
}

type ActionReq struct {
	Payload struct {
		Devices []*DevReq `json:"devices"`
	} `json:"payload"`
}

type StateReq struct {
	Devices []*DevReq `json:"devices"`
}

func NewLight(id, name, room string) *Device {
	return &Device{
		Id:           id,
		Name:         strings.ReplaceAll(id, "_", " "),
		Description:  name,
		Room:         room,
		Type:         "devices.types.light",
		Capabilities: []*Capability{{Type: "devices.capabilities.on_off"}},
	}
}

func NewSwitch(id, name, room string) *Device {
	return &Device{
		Id:          id,
		Name:        strings.ReplaceAll(id, "_", " "),
		Description: name,
		Room:        room,
		Type:        "devices.types.switch",
		Capabilities: []*Capability{{
			Type: "devices.capabilities.on_off",
		}},
	}
}

func (c *Capability) GetVal(name string) (any, bool) {
	if c == nil || c.State == nil {
		return nil, false
	}

	if c.State.Instance == name {
		return c.State.Value, true
	}

	return nil, false
}

func (c *Capability) SetVal(name string, val any) {
	if c == nil {
		return
	}
	c.State = &State{
		Instance: name,
		Value:    val,
	}
}

func (c *Capability) SetValOk(name string, val any) {
	if c == nil {
		return
	}
	c.State = &State{
		Instance:     name,
		Value:        val,
		ActionResult: &ActionResult{Status: "DONE"},
	}
}

func (c *Capability) GetBool(name string) (bool, bool) {
	if c == nil || c.State == nil {
		return false, false
	}

	if c.State.Instance == name {
		if b, ok := c.State.Value.(bool); ok {
			return b, true
		}
	}

	return false, false
}
