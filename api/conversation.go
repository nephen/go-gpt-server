package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-gpt-server/cachecenter"
	_ "go-gpt-server/env"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

const model = "text-davinci-002-render-sha"
const CONVERSATION_NUM = 5

var (
	mutex            sync.Mutex
	chatGPTClient    *resty.Client
	firstBoot        bool
	sessionLocker    *SessionLocker
	MultiSession     bool
	conversationIds  []string
	conversationUsed map[string]bool
)

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

type SessionLocker struct {
	locks map[string]*sync.Mutex
}

func NewSessionLocker() *SessionLocker {
	return &SessionLocker{
		locks: make(map[string]*sync.Mutex),
	}
}

func (s *SessionLocker) Lock(sessionID string) bool {
	lock, ok := s.locks[sessionID]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[sessionID] = lock
	}
	return lock.TryLock()
}

func (s *SessionLocker) Unlock(sessionID string) {
	lock, ok := s.locks[sessionID]
	if !ok {
		panic(fmt.Sprintf("trying to unlock a non-existing session lock: %s", sessionID))
	}
	lock.Unlock()
}

func init() {
	chatGPTClient = resty.New().SetBaseURL("http://" + os.Getenv("UNOFFICIAL_PROXY"))
	chatGPTClient.SetHeader("Authorization", os.Getenv("ACCESS_TOKEN"))
	firstBoot = true
	MultiSession = false

	conversationIds = make([]string, CONVERSATION_NUM)
	conversationUsed = make(map[string]bool)

	// Create a new session locker
	sessionLocker = NewSessionLocker()
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

func randomNum(conversationIds []string, conversationUsed map[string]bool, total int) (int, error) {
	rand.Seed(time.Now().UnixNano())
	unused := []int{}
	for i, id := range conversationIds {
		if !conversationUsed[id] && i < total {
			unused = append(unused, i)
		}
	}
	if len(unused) == 0 {
		return 0, errors.New("会话被占完")
	}
	index := rand.Intn(len(unused))
	return unused[index], nil
}

func getConversationIndex(total int) int {
	if !MultiSession {
		return 0
	}
	index := -1
	for i := 0; i < len(conversationIds); i++ {
		var err error
		index, err = randomNum(conversationIds, conversationUsed, total)
		if err != nil {
			fmt.Println(err.Error())
			return -1
		}
		fmt.Printf("取出的会话index是：%d\n", index)
		break
	}
	return index
}

func handleResBody(resBody []byte) (int, string) {
	// 将JSON字符串反序列化为一个map
	var data map[string]interface{}
	err := json.Unmarshal(resBody, &data)
	if err != nil {
		fmt.Println(err.Error())
	}

	total := int(data["total"].(float64))
	items := data["items"]
	if items != nil {
		println("before lock")
		// mutex.Lock()
		// defer mutex.Unlock()
		for k, v := range items.([]interface{}) {
			fmt.Printf("itemsData %v, %v\n", k, v.(map[string]interface{})["id"])
			conversationIds = append(conversationIds, v.(map[string]interface{})["id"].(string))
		}
		println("after lock")
	}
	index := getConversationIndex(total)
	// 在map中添加一个字段
	data["index"] = index
	data["multi"] = MultiSession

	// 将map重新序列化为JSON字符串
	resBodyNew, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
	}

	return total, string(resBodyNew)
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
		if apiType == "conversations" {
			// 获取 URL 指针
			u := r.URL

			// 检查 URL 是否包含指定参数
			q := u.Query()
			if q.Get("limit") == "" {
				// 如果 URL 中不包含指定参数，则添加该参数
				q.Set("limit", strconv.Itoa(CONVERSATION_NUM))
				u.RawQuery = q.Encode()
			}
			u.RawQuery = q.Encode()
			key = r.URL.String() + ":" + apiType
			println(key)
		}
		value, found := cachecenter.C.Get(key)
		if found {
			var valueString string
			if apiType == "conversations" {
				_, valueString = handleResBody([]byte(value.(string)))
			} else {
				valueString = value.(string)
			}
			fmt.Printf("get key: %v, value:%v\n", key, valueString)
			fmt.Fprintf(w, valueString)
			return
		}
	}

	target := os.Getenv("UNOFFICIAL_PROXY")

	var reqBody string
	conversationIdBak := "empty"
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
		// 换行转义
		conversation.Content = strings.ReplaceAll(conversation.Content, "\n", "\\n")
		if conversation.ConversationID != "" {
			conversationIdBak = conversation.ConversationID
		}

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

	// 对单独的会话加锁
	sessionId := "single"
	if MultiSession {
		sessionId = conversationIdBak
	}
	getLocked := false
	if r.Method == "POST" {
		if sessionLocker.Lock(sessionId) {
			getLocked = true
			// mutex.Lock()
			// defer mutex.Unlock()
			conversationUsed[conversationIdBak] = true
		} else {
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	defer func() {
		if getLocked {
			// mutex.Lock()
			// defer mutex.Unlock()
			sessionLocker.Unlock(sessionId)
			conversationUsed[conversationIdBak] = false
		}
	}()

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
					var total int
					total, bodyValue = handleResBody(resBody)
					if total == 0 { // 不需要缓存
						fmt.Println("total 0, no need cache", bodyValue)
						needCache = false
						firstBoot = false
					} else if firstBoot || (MultiSession && total < CONVERSATION_NUM) { // 不需要缓存
						bodyValue = `{"items":[],"total":0,"limit":1,"offset":0,"has_missing_conversations":false}`
						needCache = false
						firstBoot = false
						fmt.Printf("first boot or total is %d, no need cache\n", total)
					}
					println("bodyValue:", bodyValue)
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
				res.Header.Set("Content-Length", strconv.Itoa(len(bodyValue)))
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
