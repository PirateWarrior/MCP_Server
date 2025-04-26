package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// FofaClient FOFA API客户端结构体
type FofaClient struct {
	Email   string
	Key     string
	BaseURL string
}

// NewFofaClient 创建新的FOFA客户端
func NewFofaClient(email, key string) *FofaClient {
	return &FofaClient{
		Email:   email,
		Key:     key,
		BaseURL: "https://fofa.info/api/v1",
	}
}

// SearchResult FOFA搜索结果
type SearchResult struct {
	Error   bool       `json:"error"`
	ErrMsg  string     `json:"errmsg"`
	Mode    string     `json:"mode"`
	Page    int        `json:"page"`
	Size    int        `json:"size"`
	Results [][]string `json:"results"`
}

// Search 执行FOFA搜索
func (fc *FofaClient) Search(query string, page, size int, fields string) (*SearchResult, error) {
	params := url.Values{}
	params.Add("qbase64", base64Encode(query))
	params.Add("email", fc.Email)
	params.Add("key", fc.Key)
	params.Add("page", fmt.Sprintf("%d", page))
	params.Add("size", fmt.Sprintf("%d", size))
	params.Add("fields", fields)

	resp, err := http.Get(fmt.Sprintf("%s/search/all?%s", fc.BaseURL, params.Encode()))
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

	return &result, nil
}

// 辅助函数: base64编码
func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// FofaSearchArguments FOFA搜索工具参数
type FofaSearchArguments struct {
	Query  string `json:"query" jsonschema:"required,description=搜索查询语句，支持以下查询语法:\n1. 基础查询: title=\"百度\"\n2. IP查询: ip=\"1.1.1.1\"\n3. 端口查询: port=\"80\"\n4. 协议查询: protocol=\"http\"\n5. 国家查询: country=\"CN\"\n6. 域名查询: domain=\"qq.com\"\n7. 操作系统查询: os=\"windows\"\n8. 服务器查询: server=\"nginx\"\n9. ICP备案查询: icp=\"京ICP备\"\n10. 证书查询: cert=\"*.example.com\"\n11. 组合查询: title=\"admin\" && country=\"US\"\n12. 时间范围查询: after=\"2023-01-01\" && before=\"2023-12-31\"\n13. 正则查询: title=~\"admin.*\"\n14. 模糊查询: title=*\"管理后台\"\n15. 排除查询: !title=\"test\"\n示例: title=\"管理后台\" && country=\"CN\""`
	Page   int    `json:"page" jsonschema:"required,description=页码，默认为1"`
	Size   int    `json:"size" jsonschema:"required,description=每页数量，默认为50"`
	Fields string `json:"fields" jsonschema:"required,description=返回字段(逗号分隔)，可选值:\n1. ip: IP地址\n2. port: 端口\n3. protocol: 协议名\n4. country: 国家代码\n5. country_name: 国家名\n6. region: 区域\n7. city: 城市\n8. longitude: 经度\n9. latitude: 纬度\n10. asn: ASN编号\n11. org: ASN组织\n12. host: 主机名\n13. domain: 域名\n14. os: 操作系统\n15. server: 网站server\n16. icp: ICP备案号\n17. title: 网站标题\n18. jarm: JARM指纹\n19. header: 网站header\n20. banner: 协议banner\n21. cert: 证书\n22. base_protocol: 基础协议\n23. link: 资产URL\n24-50: 其他专业版/商业版字段\n示例: fields=ip,port,title\n默认: host,ip,port"`
}

func main() {
	var email, key string
	flag.StringVar(&email, "email", "", "FOFA API email")
	flag.StringVar(&key, "key", "", "FOFA API key")
	flag.Parse()

	if email == "" || key == "" {
		fmt.Println("Error: email and key are required")
		fmt.Println("Usage: go run fofa_mcp_server.go -email <your_email> -key <your_key>")
		os.Exit(1)
	}

	done := make(chan struct{})

	server := mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := server.RegisterTool("fofa_search", "FOFA搜索引擎", func(arguments FofaSearchArguments) (*mcp_golang.ToolResponse, error) {
		// 为 Page 和 Size 设置默认值
		if arguments.Page == 0 {
			arguments.Page = 1
		}
		if arguments.Size == 0 {
			arguments.Size = 50
		}
		client := NewFofaClient(email, key)
		result, err := client.Search(arguments.Query, arguments.Page, arguments.Size, arguments.Fields)
		if err != nil {
			return nil, err
		}
		if result.Error {
			return nil, fmt.Errorf(result.ErrMsg)
		}

		var output strings.Builder
		fmt.Fprintf(&output, "搜索结果(共%d条):\n", len(result.Results))
		for _, item := range result.Results {
			fmt.Fprintln(&output, strings.Join(item, " | "))
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
