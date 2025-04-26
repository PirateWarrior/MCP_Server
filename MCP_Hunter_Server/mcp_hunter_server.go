package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// HunterClient Hunter API客户端结构体
type HunterClient struct {
	APIKey  string
	BaseURL string
}

// NewHunterClient 创建新的Hunter客户端
func NewHunterClient(apiKey string) *HunterClient {
	return &HunterClient{
		APIKey:  apiKey,
		BaseURL: "https://hunter.qianxin.com/openApi/search",
	}
}

// SearchResult Hunter搜索结果
type SearchResult struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    *ResultData `json:"data"`
}

type ResultData struct {
	Total        int           `json:"total"`
	Time         int           `json:"time"`
	Arr          []interface{} `json:"arr"`
	ConsumeQuota string        `json:"consume_quota"`
	RestQuota    string        `json:"rest_quota"`
}

// Search 执行Hunter搜索
func (hc *HunterClient) Search(query string, page, size int, isWeb int, startTime, endTime string) (*SearchResult, error) {
	params := url.Values{}
	params.Add("api-key", hc.APIKey)
	params.Add("search", base64URLEncode(query))
	params.Add("page", fmt.Sprintf("%d", page))
	params.Add("page_size", fmt.Sprintf("%d", size))
	params.Add("is_web", fmt.Sprintf("%d", isWeb))

	if startTime != "" {
		params.Add("start_time", startTime)
	}
	if endTime != "" {
		params.Add("end_time", endTime)
	}

	resp, err := http.Get(fmt.Sprintf("%s?%s", hc.BaseURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	if result.Code != 200 {
		return nil, fmt.Errorf(result.Message)
	}

	return &result, nil
}

// 辅助函数: base64URL编码
func base64URLEncode(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

// HunterSearchArguments Hunter搜索工具参数
type HunterSearchArguments struct {
	Query     string `json:"query" jsonschema:"required,description=搜索查询语句。支持以下语法:\n- IP搜索: ip=\"1.1.1.1\" 或 ip=\"220.181.111.0/24\"\n- 端口搜索: ip.port=\"80\"\n- 域名搜索: domain=\"example.com\"\n- Web标题搜索: web.title=\"登录页面\"\n- 响应头搜索: header.server=\"Microsoft-IIS/10\"\n- 资产类型: is_web=true(web资产)\n示例: ip=\"1.1.1.1\" && ip.port=\"80\""`
	Page      int    `json:"page" jsonschema:"required,description=页码，默认为1"`
	Size      int    `json:"size" jsonschema:"required,description=每页数量，默认为20"`
	IsWeb     int    `json:"is_web" jsonschema:"required,description=资产类型: 1(web资产), 2(非web资产), 3(全部)"`
	StartTime string `json:"start_time" jsonschema:"description=开始时间，格式为YYYY-MM-DD"`
	EndTime   string `json:"end_time" jsonschema:"description=结束时间，格式为YYYY-MM-DD"`
}

func main() {
	apiKey := flag.String("key", "", "Hunter API key")
	flag.Parse()

	if *apiKey == "" {
		panic("API key is required, please provide it via --key parameter")
	}

	done := make(chan struct{})

	server := mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := server.RegisterTool("hunter_search", "Hunter搜索引擎", func(arguments HunterSearchArguments) (*mcp_golang.ToolResponse, error) {
		// 为参数设置默认值
		if arguments.Page == 0 {
			arguments.Page = 1
		}
		if arguments.Size == 0 {
			arguments.Size = 20
		}
		if arguments.IsWeb == 0 {
			arguments.IsWeb = 1
		}

		client := NewHunterClient(*apiKey)
		result, err := client.Search(arguments.Query, arguments.Page, arguments.Size, arguments.IsWeb, arguments.StartTime, arguments.EndTime)
		if err != nil {
			return nil, err
		}

		var output strings.Builder
		fmt.Fprintf(&output, "搜索结果(共%d条):\n", result.Data.Total)
		for _, item := range result.Data.Arr {
			itemMap := item.(map[string]interface{})
			fmt.Fprintf(&output, "IP: %s | 端口: %v | 标题: %s\n",
				itemMap["ip"], itemMap["port"], itemMap["web_title"])
		}

		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(output.String())), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.Serve()
	if err != nil {
		panic(err)
	}

	<-done
}
