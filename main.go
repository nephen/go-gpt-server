package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

const model = "text-davinci-002-render-sha"
const chat_code = "dyhlyb"

var c *cache.Cache

type message_struct struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chat_struct struct {
	Model    string           `json:"model"`
	Stream   bool             `json:"stream"`
	Messages []message_struct `json:"messages"`
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

// 删除匹配某个前缀的缓存项
func deleteCacheByPrefix(c *cache.Cache, prefix string) {
	keys := c.Items()
	for k := range keys {
		if strings.HasPrefix(k, prefix) {
			c.Delete(k)
			println("delete cache: ", k)
		}
	}
}

func enableCors(w *http.Response) {
	(*w).Header.Set("Access-Control-Allow-Origin", "*")
	(*w).Header.Set("Access-Control-Allow-Headers", "*")
	(*w).Header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	(*w).Header.Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
	(*w).Header.Set("Access-Control-Allow-Credentials", "true")
}

func enableCors2(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
}

func init() {
	// 设置超时时间和清理时间
	c = cache.New(5*time.Minute, 10*time.Minute)
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	appId := r.URL.Query()["appId"]
	page := r.URL.Query()["page"]
	pageSize := r.URL.Query()["pageSize"]

	enableCors2(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Panic("server not support")
	}
	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Second)
		fmt.Fprintf(w, "data: %d%s%s%s\n\n", i, appId[0], page[0], pageSize[0])
		flusher.Flush()
	}
	fmt.Fprintf(w, "event: close\ndata: close\n\n") // 一定要带上data，否则无效
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	for k, v := range r.Header {
		if k == "Authorization" {
			fmt.Println(k, v)
			if v[0] != string(chat_code) {
				enableCors2(&w)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			r.Header.Del(k)
		}
	}
	if r.Method == "OPTIONS" {
		enableCors2(&w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var chat chat_struct
	// Read body
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		fmt.Println("ReadAll", err.Error())
	}
	// Unmarshal
	err = json.Unmarshal(b, &chat)
	if err != nil {
		fmt.Println("Unmarshal", err.Error())
	}
	fmt.Println("chat:", chat)

	target := "107.148.26.186:5566"
	director := func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = target
		req.URL.Path = "/v1/chat/completions"
		req.Host = target
		b, _ := json.Marshal(chat)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		req.ContentLength = (int64)(len(b))
		req.Header.Set("Authorization", "Bearer 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")
	}
	response := func(res *http.Response) error {
		enableCors(res)
		return nil
	}
	proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: response}
	proxy.ServeHTTP(w, r)
}

func handleConv(w http.ResponseWriter, r *http.Request) {
	var apiType string
	for k, v := range r.Header {
		if k == "Authorization" {
			fmt.Println(k, v)
			if v[0] != string(chat_code) {
				enableCors2(&w)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			r.Header.Del(k)
		} else if k == "Chat-Type" {
			fmt.Println(k, v)
			apiType = v[0]
			r.Header.Del(k)
		}
		// fmt.Println(k, v)
	}

	if r.Method == "OPTIONS" {
		enableCors2(&w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if apiType == "" {
		enableCors2(&w)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := r.URL.String() + ":" + apiType
	println(key)

	if r.Method == "GET" {
		value, found := c.Get(key)
		if found {
			fmt.Printf("get key: %v, value:%v\n", key, value)
			enableCors2(&w)
			fmt.Fprintf(w, value.(string))
			return
		}
	}

	target := "107.148.26.186:8080"

	var body string
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
		conversationIdBak = conversation.ConversationID

		body = fmt.Sprintf(`
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
		if body != "" {
			req.ContentLength = (int64)(len(body))
			req.Body = ioutil.NopCloser(bytes.NewBufferString(body))
		}
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6Ik1UaEVOVUpHTkVNMVFURTRNMEZCTWpkQ05UZzVNRFUxUlRVd1FVSkRNRU13UmtGRVFrRXpSZyJ9.eyJodHRwczovL2FwaS5vcGVuYWkuY29tL3Byb2ZpbGUiOnsiZW1haWwiOiI5OTUxNjg2OTRAcXEuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWV9LCJodHRwczovL2FwaS5vcGVuYWkuY29tL2F1dGgiOnsidXNlcl9pZCI6InVzZXItRkVhUkxEempOWnp6V29SSkFDdm1vN25SIn0sImlzcyI6Imh0dHBzOi8vYXV0aDAub3BlbmFpLmNvbS8iLCJzdWIiOiJhdXRoMHw2Mzk1Mzk3N2MzM2JhNGYyMjA3ZDQ3ZGUiLCJhdWQiOlsiaHR0cHM6Ly9hcGkub3BlbmFpLmNvbS92MSIsImh0dHBzOi8vb3BlbmFpLm9wZW5haS5hdXRoMGFwcC5jb20vdXNlcmluZm8iXSwiaWF0IjoxNjgwMzEwMzkzLCJleHAiOjE2ODE1MTk5OTMsImF6cCI6IlRkSkljYmUxNldvVEh0Tjk1bnl5d2g1RTR5T282SXRHIiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCBtb2RlbC5yZWFkIG1vZGVsLnJlcXVlc3Qgb3JnYW5pemF0aW9uLnJlYWQgb2ZmbGluZV9hY2Nlc3MifQ.uhOhEAygPUTvxaDvC_jXq5sSFM_Rgx9vkb-vTrGAFePOs7On2fzJojeH0VlBc8JC_CpkWtPY__uNDYfuMSiQLwgqEGCYRsoANDPNRWHLBAhX9x-X-isiR6F49cF6V58xcGrcLW1GPcEWHlINMZZHGkOsX_Wju3q36Qmb4LBaCa1m6gQU1K5Xq_z8FAoYhuqXdqvYimSCiRc4gTrKpWZ4BtX-yutepY5VnpkeHTc7RAvFrlgjIzWSBEWs9hqfD1LKMp89gC0qLtBNcX2qKMRvLFzvAjhFeEKCe6-xITOndJ5XBkn1DFOYBG_QJedm9YW-pl3GUksQhHoYDj_0CjwuLg")
	}
	response := func(res *http.Response) error {
		enableCors(res)
		// fmt.Println(string(body))
		fmt.Println("status: " + res.Status)
		if res.StatusCode == 200 {
			if r.Method == "GET" {
				// 在这里获取 HTTP 响应体
				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return err
				}
				cacheValue := string(body)
				if apiType != "conversations" {
					var conversationItems conversation_items_struct
					// Unmarshal
					err = json.Unmarshal(body, &conversationItems)
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
					cacheValue = conversationItemsJsonStr
				}
				// 注意：必须把响应体重新设置回去，否则客户端无法接收到数据
				res.Body = ioutil.NopCloser(bytes.NewBufferString(cacheValue))
				res.ContentLength = (int64)(len(cacheValue))
				c.Set(key, cacheValue, cache.NoExpiration)
				fmt.Printf("cached key: %v, value: %v", key, cacheValue)
			} else {
				// deleteCacheByPrefix(c, "/conv?")
				key := "/conv:conversation/" + conversationIdBak
				value, found := c.Get(key)
				if found {
					reader := bufio.NewReader(res.Body)
					message, err := reader.ReadString('\n')
					if err != nil {
						log.Println(err)
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
								err = json.Unmarshal([]byte(valueString), &conversationItems)
								if err != nil {
									fmt.Println("conversationItems Unmarshal", err.Error())
								} else {
									fmt.Println("conversationItems:", conversationItems)

									conversationItems.CurrentNode = conversationMsgs.Message.Id
									// Marshal
									conversationItemsJson, err := json.Marshal(conversationItems)
									if err != nil {
										fmt.Println("Failed to convert to JSON:", err.Error())
									} else {
										c.Set(key, string(conversationItemsJson), cache.NoExpiration)
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

func main() {
	http.Handle("/event", http.HandlerFunc(handleSSE))
	http.Handle("/chat", http.HandlerFunc(handleChat))
	http.Handle("/conv", http.HandlerFunc(handleConv))
	http.ListenAndServe(":8000", nil)
}
