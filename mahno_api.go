package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Item struct {
	Name           string         `json:"name"`
	Type_          string         `json:"type"`
	HumanName      string         `json:"human_name"`
	Changed        time.Time      `json:"changed"`
	Checked        time.Time      `json:"checked"`
	Value          string         `json:"value"`
	RawValue       any            `json:"raw_value"`
	FormattedValue string         `json:"formatted_value"`
	Good           bool           `json:"good"`
	UI             bool           `json:"ui"`
	Tags           []string       `json:"tags,omitempty"`
	Groups         []string       `json:"groups,omitempty"`
	Meta           map[string]any `json:"meta,omitempty"`
}

type MahnoApi interface {
	ItemCommand(item string, cmd string) error
	SetItemState(item string, val string) error
	AllItems() ([]*Item, error)
}

type MahnoHttpApi struct {
	host   string
	client *http.Client
	logger *zap.SugaredLogger
}

func NewMahnoApi(logger *zap.SugaredLogger, host string) *MahnoHttpApi {
	client := &http.Client{Timeout: time.Second * 3}
	return &MahnoHttpApi{host: host, client: client, logger: logger}
}

func (m *MahnoHttpApi) SetLogger(logger *zap.SugaredLogger) {
	m.logger = logger
}

func (m *MahnoHttpApi) doReqReader(method string, path string, data string) (io.ReadCloser, error) {
	url := "http://" + m.host + path

	m.logger.Debugf("url: %s", url)

	var req *http.Request
	var err error

	if data != "" {
		req, err = http.NewRequest(method, url, bytes.NewBuffer([]byte(data)))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (m *MahnoHttpApi) doReq(method string, path string, data string) ([]byte, error) {
	b, err := m.doReqReader(method, path, data)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	res, err := ioutil.ReadAll(b)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MahnoHttpApi) ItemCommand(item string, cmd string) error {
	body, err := m.doReq("POST", "/items/"+item, cmd)

	if err != nil {
		m.logger.Errorf("error talking to mahno: %s", err.Error())
		return err
	}

	m.logger.Infof(fmt.Sprintf("body: %s", body))
	return nil
}

func (m *MahnoHttpApi) SetItemState(item string, val string) error {
	body, err := m.doReq("POST", "/items/"+item, val)

	if err != nil {
		m.logger.Errorf("error talking to mahno: %s", err.Error())
		return err
	}

	m.logger.Infof(fmt.Sprintf("body: %s", body))
	return nil
}

func (m *MahnoHttpApi) AllItems() ([]*Item, error) {
	body, err := m.doReqReader("GET", "/items", "")

	if err != nil {
		m.logger.Errorf("error talking to mahno: %s", err.Error())
		return nil, err
	}

	defer body.Close()
	var res []*Item
	decoder := json.NewDecoder(body)

	if err = decoder.Decode(&res); err != nil {
		m.logger.Errorf("can't decode: %v", err)
		return nil, err
	}

	return res, nil
}
