package v2board

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/XrayR-project/XrayR/api"
	"github.com/bitly/go-simplejson"
	"github.com/go-resty/resty/v2"
)

// APIClient create a api client to the panel.
type APIClient struct {
	client      *resty.Client
	APIHost     string
	NodeID      int
	Key         string
	NodeType    string
	EnableVless bool
	EnableXTLS  bool
}

// New creat a api instance
func New(apiConfig *api.Config) *APIClient {

	client := resty.New()
	client.SetRetryCount(3)
	if apiConfig.Timeout > 0 {
		client.SetTimeout(time.Duration(apiConfig.Timeout) * time.Second)
	} else {
		client.SetTimeout(5 * time.Second)
	}
	client.OnError(func(req *resty.Request, err error) {
		if v, ok := err.(*resty.ResponseError); ok {
			// v.Response contains the last response from the server
			// v.Err contains the original error
			log.Print(v.Err)
		}
	})
	client.SetHostURL(apiConfig.APIHost)
	// Create Key for each requests
	client.SetQueryParam("key", apiConfig.Key)
	client.SetQueryParams(map[string]string{
		"node_id":    strconv.Itoa(apiConfig.NodeID),
		"token":      apiConfig.Key,
		"local_port": "1",
	})
	apiClient := &APIClient{
		client:      client,
		NodeID:      apiConfig.NodeID,
		Key:         apiConfig.Key,
		APIHost:     apiConfig.APIHost,
		NodeType:    apiConfig.NodeType,
		EnableVless: apiConfig.EnableVless,
		EnableXTLS:  apiConfig.EnableXTLS,
	}
	return apiClient
}

// Describe return a description of the client
func (c *APIClient) Describe() api.ClientInfo {
	return api.ClientInfo{APIHost: c.APIHost, NodeID: c.NodeID, Key: c.Key, NodeType: c.NodeType}
}

// Debug set the client debug for client
func (c *APIClient) Debug() {
	c.client.SetDebug(true)
}

func (c *APIClient) assembleURL(path string) string {
	return c.APIHost + path
}

func (c *APIClient) parseResponse(res *resty.Response, path string, err error) (*simplejson.Json, error) {
	if err != nil {
		return nil, fmt.Errorf("request %s failed: %s", c.assembleURL(path), err)
	}

	if res.StatusCode() > 400 {
		body := res.Body()
		return nil, fmt.Errorf("request %s failed: %s, %s", c.assembleURL(path), string(body), err)
	}
	rtn, err := simplejson.NewJson(res.Body())
	if err != nil {
		return nil, fmt.Errorf("Ret %s invalid", res.String())
	}
	return rtn, nil
}

// GetNodeInfo will pull NodeInfo Config from sspanel
func (c *APIClient) GetNodeInfo() (nodeInfo *api.NodeInfo, err error) {
	var path string
	switch c.NodeType {
	case "V2ray":
		path = "api/v1/server/Deepbwork/config"
	case "Trojan":
		path = "api/v1/server/TrojanTidalab/config"
	case "Shadowsocks":
		if nodeInfo, err = c.ParseSSNodeResponse(); err == nil {
			return nodeInfo, nil
		} else {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unsupported Node type: %s", c.NodeType)
	}
	res, err := c.client.R().
		ForceContentType("application/json").
		Get(path)

	response, err := c.parseResponse(res, path, err)
	if err != nil {
		return nil, err
	}

	switch c.NodeType {
	case "V2ray":
		nodeInfo, err = c.ParseV2rayNodeResponse(response)
	case "Trojan":
		nodeInfo, err = c.ParseTrojanNodeResponse(response)
	case "Shadowsocks":
		nodeInfo, err = c.ParseSSNodeResponse()
	default:
		return nil, fmt.Errorf("Unsupported Node type: %s", c.NodeType)
	}

	if err != nil {
		res, _ := response.MarshalJSON()
		return nil, fmt.Errorf("Parse node info failed: %s", string(res))
	}

	return nodeInfo, nil
}

// GetUserList will pull user form sspanel
func (c *APIClient) GetUserList() (UserList *[]api.UserInfo, err error) {
	var path string
	switch c.NodeType {
	case "V2ray":
		path = "api/v1/server/Deepbwork/user"
	case "Trojan":
		path = "api/v1/server/TrojanTidalab/user"
	case "Shadowsocks":
		path = "api/v1/server/ShadowsocksTidalab/user"
	default:
		return nil, fmt.Errorf("Unsupported Node type: %s", c.NodeType)
	}
	res, err := c.client.R().
		SetQueryParam("node_id", strconv.Itoa(c.NodeID)).
		ForceContentType("application/json").
		Get(path)

	response, err := c.parseResponse(res, path, err)
	if err != nil {
		return nil, err
	}
	numOfUsers := len(response.Get("data").MustArray())
	userList := make([]api.UserInfo, numOfUsers)
	for i := 0; i < numOfUsers; i++ {
		user := api.UserInfo{}
		user.UID = response.Get("data").GetIndex(i).Get("id").MustInt()
		switch c.NodeType {
		case "Shadowsocks":
			user.Email = response.Get("data").GetIndex(i).Get("secret").MustString()
			user.Passwd = response.Get("data").GetIndex(i).Get("secret").MustString()
			user.Method = response.Get("data").GetIndex(i).Get("cipher").MustString()
			user.Port = response.Get("data").GetIndex(i).Get("port").MustInt()
		case "Trojan":
			user.UUID = response.Get("data").GetIndex(i).Get("trojan_user").Get("password").MustString()
			user.Email = response.Get("data").GetIndex(i).Get("trojan_user").Get("password").MustString()
		case "V2ray":
			user.UUID = response.Get("data").GetIndex(i).Get("v2ray_user").Get("uuid").MustString()
			user.Email = response.Get("data").GetIndex(i).Get("v2ray_user").Get("email").MustString()
			user.AlterID = response.Get("data").GetIndex(i).Get("v2ray_user").Get("alter_id").MustInt()
		}
		userList[i] = user
	}
	return &userList, nil
}

// ReportUserTraffic reports the user traffic
func (c *APIClient) ReportUserTraffic(userTraffic *[]api.UserTraffic) error {
	var path string
	switch c.NodeType {
	case "V2ray":
		path = "api/v1/server/Deepbwork/submit"
	case "Trojan":
		path = "api/v1/server/TrojanTidalab/submit"
	case "Shadowsocks":
		path = "api/v1/server/ShadowsocksTidalab/submit"
	}

	data := make([]UserTraffic, len(*userTraffic))
	for i, traffic := range *userTraffic {
		data[i] = UserTraffic{
			UID:      traffic.UID,
			Upload:   traffic.Upload,
			Download: traffic.Download}
	}

	res, err := c.client.R().
		SetQueryParam("node_id", strconv.Itoa(c.NodeID)).
		SetBody(data).
		ForceContentType("application/json").
		Post(path)
	_, err = c.parseResponse(res, path, err)
	if err != nil {
		return err
	}
	return nil
}

// GetNodeRule implements the API interface
func (c *APIClient) GetNodeRule() (*[]api.DetectRule, error) {
	ruleList := make([]api.DetectRule, 0)
	if c.NodeType != "V2ray" {
		return &ruleList, nil
	}

	// V2board only support the rule for v2ray
	path := "api/v1/server/Deepbwork/config"
	res, err := c.client.R().
		ForceContentType("application/json").
		Get(path)

	response, err := c.parseResponse(res, path, err)
	if err != nil {
		return nil, err
	}
	ruleListResponse := response.Get("routing").Get("rules").GetIndex(1).Get("domain").MustStringArray()
	for i, rule := range ruleListResponse {
		ruleListItem := api.DetectRule{
			ID:      i,
			Pattern: rule,
		}
		ruleList = append(ruleList, ruleListItem)
	}
	return &ruleList, nil
}

// ReportNodeStatus implements the API interface
func (c *APIClient) ReportNodeStatus(nodeStatus *api.NodeStatus) (err error) {
	return nil
}

//ReportNodeOnlineUsers implements the API interface
func (c *APIClient) ReportNodeOnlineUsers(onlineUserList *[]api.OnlineUser) error {
	return nil
}

// ReportIllegal implements the API interface
func (c *APIClient) ReportIllegal(detectResultList *[]api.DetectResult) error {
	return nil
}

// ParseTrojanNodeResponse parse the response for the given nodeinfor format
func (c *APIClient) ParseTrojanNodeResponse(nodeInfoResponse *simplejson.Json) (*api.NodeInfo, error) {
	var TLSType = "tls"
	if c.EnableXTLS {
		TLSType = "xtls"
	}
	port := nodeInfoResponse.Get("local_port").MustInt()
	host := nodeInfoResponse.Get("ssl").Get("sni").MustString()

	// Create GeneralNodeInfo
	nodeinfo := &api.NodeInfo{
		NodeType:          c.NodeType,
		NodeID:            c.NodeID,
		Port:              port,
		TransportProtocol: "tcp",
		EnableTLS:         true,
		TLSType:           TLSType,
		Host:              host,
	}
	return nodeinfo, nil
}

// ParseSSNodeResponse parse the response for the given nodeinfor format
func (c *APIClient) ParseSSNodeResponse() (*api.NodeInfo, error) {
	var port int
	var method string
	userInfo, err := c.GetUserList()
	if err != nil {
		return nil, err
	}
	if len(*userInfo) > 0 {
		port = (*userInfo)[0].Port
		method = (*userInfo)[0].Method
	}

	// Create GeneralNodeInfo
	nodeinfo := &api.NodeInfo{
		NodeType:          c.NodeType,
		NodeID:            c.NodeID,
		Port:              port,
		TransportProtocol: "tcp",
		CypherMethod:      method,
	}

	return nodeinfo, nil
}

// ParseV2rayNodeResponse parse the response for the given nodeinfor format
func (c *APIClient) ParseV2rayNodeResponse(nodeInfoResponse *simplejson.Json) (*api.NodeInfo, error) {
	var TLSType string = "tls"
	var path, host string
	var enableTLS bool
	var alterID int = 0
	if c.EnableXTLS {
		TLSType = "xtls"
	}
	inboundInfo := nodeInfoResponse.Get("inbound")
	port := inboundInfo.Get("port").MustInt()
	transportProtocol := inboundInfo.Get("streamSettings").Get("network").MustString()

	switch transportProtocol {
	case "ws":
		path = inboundInfo.Get("streamSettings").Get("wsSettings").Get("path").MustString()
		host = inboundInfo.Get("streamSettings").Get("wsSettings").Get("headers").Get("Host").MustString()
	}

	if inboundInfo.Get("streamSettings").Get("security").MustString() == "tls" {
		enableTLS = true
	} else {
		enableTLS = false
	}

	userInfo, err := c.GetUserList()
	if err != nil {
		return nil, err
	}
	if len(*userInfo) > 0 {
		alterID = (*userInfo)[0].AlterID
	}
	// Create GeneralNodeInfo
	nodeinfo := &api.NodeInfo{
		NodeType:          c.NodeType,
		NodeID:            c.NodeID,
		Port:              port,
		AlterID:           alterID,
		TransportProtocol: transportProtocol,
		EnableTLS:         enableTLS,
		TLSType:           TLSType,
		Path:              path,
		Host:              host,
		EnableVless:       c.EnableVless,
	}
	return nodeinfo, nil
}
