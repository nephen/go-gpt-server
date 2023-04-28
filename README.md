### 编译方式
```
GOARCH=amd64 GOOS=linux go build
```

### 环境变量
编辑根目录下面的.env文件
```sh
API_PROXY= # api代理地址
CHAT_API_KEY= # apikey
UNOFFICIAL_PROXY= # 非官方代理地址
ACCESS_TOKEN= # 非官方accessToken
SECRET_KEY= # 生产token的key
```

### 运行方式
设置完根目录下的.env文件后，直接运行二进制文件即可，如果要走域名来访问接口，需要自己搭建nginx反向代理，配置如下。
```sh
cp .env.example .env
nohup ./go-gpt-server > server.log 2>&1 &
```
nginx代理配置：
```
server {
	listen 80;
	listen [::]:80;

	server_name YOUR_DOMAIN;
	location / {
		proxy_buffering off;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Host $http_host;
        proxy_pass http://127.0.0.1:8000;
	}
}
```

### 接口调用
具体调用方式见html目录下提供的sample案例，实际项目参考[openai-quickstart-vue](https://github.com/nephen/openai-quickstart-vue)。
1. token
生成一定期限的token，供客户端接口调用时使用。
    ```sh
    curl -v http://localhost:8000/token\?id\=1000\&hour\=10\&key\=YOUR_KEY
    ```

2. sse测试
http://localhost:8000/event?appId=1&page=4&pageSize=5
    ```js
    sse: function () {
        let url = 'http://localhost:8000/event?appId=1&page=4&pageSize=5'
        var source = new SSE(url, {
            headers: { 'Content-Type': 'application/json' },
        });
        this.answer = ''
        source.addEventListener('message', e => {
            console.log(e.data)
        });
        source.addEventListener('close', e => {
            console.log(e.data)
        });
        source.stream();
    }
    ```
3. conv
分三种，用于获取会话ID、父消息ID、以及GPT交互，需要有会话ID和父消息ID才能进行交互，所以第一二种是初始化的前提。
    ```js
    // GET /conv?offset=0&limit=1
    // HEADER: authorization: token, 'Chat-type': 'conversations'
    var payload = JSON.parse(e.target.response);
    if (payload !== undefined) {
        this.conversationId = payload.items[0].id
    }

    // GET /conv
    // HEADER: authorization: token, 'Chat-type': 'conversation/' + conversationId
    var payload = JSON.parse(e.target.response);
    if (payload !== undefined) {
        this.parentMessageID = payload.current_node
    }

    // POST /conv
    // HEADER: authorization: token, 'Chat-type': 'conversation', 'Accept': 'text/event-stream'
    // BODY: "content": question, "parent_message_id": parentMessageID, "conversation_id": conversationId
    if (e.data != '[DONE]') {
        var payload = JSON.parse(e.data);
        if (payload !== undefined) {
            this.conversationId = payload.conversation_id
            this.parentMessageID = payload.message.id
            this.answer = payload.message.content.parts[0]
        }
    }
    ```

4. chat
原生API接口，支持SSE流式传输。
    ```js
    let url = 'http://localhost:8000/chat'
    let body = { 'model': 'gpt-3.5-turbo', 'stream': true, 'messages': [{ 'role': 'user', 'content': this.question }] }
    var source = new SSE(url, {
        headers: { 'Content-Type': 'application/json', 'authorization': token },
        payload: JSON.stringify(body)
    });
    this.answer = ''
    source.addEventListener('message', e => {
        if (e.data !== '[DONE]') {
            var payload = JSON.parse(e.data);
            if (payload.choices[0].delta.content !== undefined && payload.choices[0].delta.content !== '') {
                this.answer += payload.choices[0].delta.content
            }
        }
    });
    source.stream();
    ```
