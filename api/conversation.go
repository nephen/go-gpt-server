package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go-gpt-server/cachecenter"
	_ "go-gpt-server/env"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

const model = "text-davinci-002-render-sha"

var (
	mutex         sync.Mutex
	chatGPTClient *resty.Client
	firstBoot     bool
)

type conversations_struct struct {
	Total int `json:"total"`
}

type conversation_struct struct {
	Content         string `json:"content"`
	ParentMessageID string `json:"parent_message_id"`
	ConversationID  string `json:"conversation_id"`
}

type conversation_items_struct struct {
	CurrentNode string  `json:"current_node"`
	Title       string  `json:"title"`
	UpdateTime  float64 `json:"update_time"`
}

type conversation_msg_struct struct {
	Id string `json:"id"`
}

type conversation_msgs_struct struct {
	ConversationID string                  `json:"conversation_id"`
	Message        conversation_msg_struct `json:"message"`
}

func init() {
	chatGPTClient = resty.New().SetBaseURL("http://" + os.Getenv("UNOFFICIAL_PROXY"))
	chatGPTClient.SetHeader("Authorization", os.Getenv("ACCESS_TOKEN"))
	firstBoot = true
}

func ClearConvs() {
	_, err := chatGPTClient.R().
		SetBody(map[string]bool{
			"is_visible": false,
		}).
		Patch("/conversations")
	if err != nil {
		println(err.Error())
		return
	}
	println("Clear all conversations")
}

func HandleConv(w http.ResponseWriter, r *http.Request) {
	apiType := r.Header.Get("Chat-Type")

	if apiType == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := r.URL.String() + ":" + apiType
	println(key)

	if r.Method == "GET" {
		value, found := cachecenter.C.Get(key)
		if found {
			fmt.Printf("get key: %v, value:%v\n", key, value)
			fmt.Fprintf(w, value.(string))
			return
		}
	}

	getLocked := false
	if r.Method == "POST" {
		if mutex.TryLock() {
			getLocked = true
		} else {
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	defer func() {
		if getLocked {
			mutex.Unlock()
		}
	}()

	target := os.Getenv("UNOFFICIAL_PROXY")

	var reqBody string
	var conversationIdBak string
	if apiType == "conversation" {
		var conversation conversation_struct
		// Read body
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			fmt.Println("ReadAll", err.Error())
		}
		// Unmarshal
		err = json.Unmarshal(b, &conversation)
		if err != nil {
			fmt.Println("Unmarshal", err.Error())
		}
		fmt.Println("conversation:", conversation)
		// 去掉换行
		conversation.Content = strings.ReplaceAll(conversation.Content, "\n", "")
		conversationIdBak = conversation.ConversationID

		// 是否需要新创建会话
		if conversation.ParentMessageID == "" || conversation.ConversationID == "" {
			conversation.ParentMessageID = uuid.NewString()
		}
		reqBody = fmt.Sprintf(`
		{
			"action": "next",
			"messages": [{
				"id": "%s",
				"author": {
					"role": "user"
				},
				"role": "user",
				"content": {
					"content_type": "text",
					"parts": ["%s"]
				}
			}],
			"parent_message_id": "%s",
			"model": "%s",
			"conversation_id": "%s"
		},`, uuid.NewString(), conversation.Content, conversation.ParentMessageID, model, conversation.ConversationID)
	}

	director := func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = target
		req.URL.Path = "/" + apiType
		req.Host = target
		if reqBody != "" {
			req.ContentLength = (int64)(len(reqBody))
			req.Body = ioutil.NopCloser(bytes.NewBufferString(reqBody))
		}
		req.Header.Set("Authorization", os.Getenv("ACCESS_TOKEN"))
	}
	getParentId := false
	response := func(res *http.Response) error {
		fmt.Println("proxy status: " + res.Status)
		if res.StatusCode == 200 {
			if r.Method == "GET" {
				// 在这里获取 HTTP 响应体
				resBody, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return err
				}
				var bodyValue string
				needCache := true
				if apiType == "conversations" {
					var conversations conversations_struct
					// Unmarshal
					err = json.Unmarshal(resBody, &conversations)
					if err != nil {
						fmt.Println("conversations Unmarshal", err.Error())
						return err
					}
					bodyValue = string(resBody)
					if conversations.Total == 0 { // 不需要缓存
						fmt.Println("total 0, no need cache", bodyValue)
						needCache = false
						firstBoot = false
					} else if firstBoot { // 不需要缓存
						bodyValue = "{\"items\":[],\"total\":0,\"limit\":1,\"offset\":0,\"has_missing_conversations\":false}"
						needCache = false
						firstBoot = false
						fmt.Println("first boot, no need cache", bodyValue)
					}
				} else {
					var conversationItems conversation_items_struct
					// Unmarshal
					err = json.Unmarshal(resBody, &conversationItems)
					if err != nil {
						fmt.Println("conversationItems Unmarshal", err.Error())
						return err
					}
					fmt.Println("conversationItems:", conversationItems)
					// Marshal
					conversationItemsJson, err := json.Marshal(conversationItems)
					if err != nil {
						fmt.Println("Failed to convert to JSON:", err)
						return err
					}
					conversationItemsJsonStr := string(conversationItemsJson)
					bodyValue = conversationItemsJsonStr
				}
				// 注意：必须把响应体重新设置回去，否则客户端无法接收到数据
				res.Body = ioutil.NopCloser(bytes.NewBufferString(bodyValue))
				res.ContentLength = (int64)(len(bodyValue))
				// 读出来后缓存起来
				if needCache {
					cachecenter.C.Set(key, bodyValue, cache.NoExpiration)
					fmt.Printf("cached key: %v, value: %v\n", key, bodyValue)
				}
			} else if r.Method == "POST" && !getParentId { // 流传输中，只要进来一次就行了
				// deleteCacheByPrefix(c, "/conv?")
				key := "/conv:conversation/" + conversationIdBak
				value, found := cachecenter.C.Get(key) // 判断有没有缓存
				if found {
					reader := bufio.NewReader(res.Body) // 需要用io方式读，不能一次性读出来
					message, err := reader.ReadString('\n')
					if err != nil {
						log.Println(err.Error())
						return err
					} else {
						firstDataByte := []byte(message)[5:]
						fmt.Println("data:", message[5:])
						var conversationMsgs conversation_msgs_struct
						// Unmarshal
						err := json.Unmarshal(firstDataByte, &conversationMsgs)
						if err != nil {
							fmt.Println("conversation_data_struct Unmarshal", err.Error())
							return err
						} else {
							fmt.Println("conversationMsgs:", conversationMsgs)
							var conversationItems conversation_items_struct
							// Unmarshal
							fmt.Println("value: ", value)
							valueString, ok := value.(string)
							if !ok {
								fmt.Println("valueString err")
							} else {
								// 将缓存的value解码成对象
								err = json.Unmarshal([]byte(valueString), &conversationItems)
								if err != nil {
									fmt.Println("conversationItems Unmarshal", err.Error())
								} else {
									fmt.Println("conversationItems:", conversationItems)
									// 更新缓存对象的值
									conversationItems.CurrentNode = conversationMsgs.Message.Id
									// Marshal
									conversationItemsJson, err := json.Marshal(conversationItems)
									if err != nil {
										fmt.Println("Failed to convert to JSON:", err.Error())
									} else {
										// 重新缓存更新后的值
										cachecenter.C.Set(key, string(conversationItemsJson), cache.NoExpiration)
										getParentId = true // 这个流连接后面不需要再进来了
									}
								}
							}
						}
					}
				}
			}
		}
		return nil
	}
	proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: response}
	proxy.ServeHTTP(w, r)
}
