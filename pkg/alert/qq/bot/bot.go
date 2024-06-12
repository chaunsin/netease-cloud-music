// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
)

type Config struct {
	Sandbox  bool
	AppId    uint64
	Secret   string
	Guild    string // 频道名称
	Username string // qq昵称
}

func (c Config) Validate() error {
	if c.AppId <= 0 {
		return errors.New("appId is empty")
	}
	if c.Secret == "" {
		return errors.New("secret is empty")
	}
	return nil
}

type Client struct {
	cfg     *Config
	api     openapi.OpenAPI
	token   *token.Token
	ws      *dto.WebsocketAP
	guild   *dto.Guild // 频道
	receive *dto.User  // 接收消息用户信息
}

func New(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("qqbot: Validate: %w", err)
	}

	// botgo.SetLogger()

	var (
		ctx      = context.Background()
		api      openapi.OpenAPI
		botToken = token.BotToken(cfg.AppId, cfg.Secret)
	)
	if cfg.Sandbox {
		// 沙箱环境
		api = botgo.NewSandboxOpenAPI(botToken).WithTimeout(3 * time.Second)
	} else {
		// 生产环境
		api = botgo.NewOpenAPI(botToken).WithTimeout(3 * time.Second)
	}

	// 获取 websocket 信息
	ws, err := api.WS(ctx, nil, "")
	if err != nil {
		return nil, fmt.Errorf("WS: %w", err)
	}

	cli := &Client{
		api:   api,
		token: botToken,
		ws:    ws,
	}
	if err := cli.init(ctx); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	go func() {
		if err := cli.Run(ctx); err != nil {
			log.Printf("[qqbot] Run err:%s\n", err)
		}
	}()
	return cli, nil
}

func (c *Client) init(ctx context.Context) error {
	// 查找当前的频道
	guilds, err := c.api.MeGuilds(ctx, &dto.GuildPager{
		Before: "",
		After:  "",
		Limit:  "100",
	})
	if err != nil {
		return fmt.Errorf("MeGuilds: %w", err)
	}
	if len(guilds) <= 0 {
		return fmt.Errorf("no guilds")
	}
	// 如果不配置频道名称,则默认取第一个频道,否则根据配置名称查找
	if c.cfg.Guild == "" {
		c.guild = guilds[0]
	} else {
		for _, g := range guilds {
			if strings.Contains(g.Name, c.cfg.Guild) {
				c.guild = g
				break
			}
		}
	}
	if c.guild == nil {
		return fmt.Errorf("no guild or not found guild")
	}

	// info, err := c.api.Guild(context.Background(), c.cfg.Guild)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("guild info:%+v\n", info)

	// 查找当前频道下接收通知用户
	members, err := c.api.GuildMembers(ctx, c.guild.ID, &dto.GuildMembersPager{
		After: "",
		Limit: "1000",
	})
	if err != nil {
		return fmt.Errorf("GuildMembers: %w", err)
	}
	var user *dto.User
	for _, v := range members {
		// TODO: 当一个频道内存在多个重名的用户时,不满足用户唯一性,从而造成消息没有发送到预期用户。
		if v.User.Username == c.cfg.Username {
			user = v.User
		}
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	c.receive = user
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	// 注册各种监听事件
	intent := event.RegisterHandlers(
		ReadyHandler(),
		ErrorNotifyHandler(),
		PrivateMessage(c.api),
		MessageAuditPass(),
		ATMessageEventHandler(c.api),
		CreateMessageHandler(),
	)

	if err := botgo.NewSessionManager().Start(c.ws, c.token, &intent); err != nil {
		return fmt.Errorf("Start: %w", err)
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	return nil
}

// Send 发送qq频道私信
// 注意: 发送私信接口有次数限制 https://bot.q.qq.com/wiki/develop/api/openapi/dms/post_dms_messages.html#%E5%8F%91%E9%80%81%E7%A7%81%E4%BF%A1
func (c *Client) Send(ctx context.Context, content string) error {
	members, err := c.api.GuildMembers(ctx, c.guild.ID, &dto.GuildMembersPager{
		After: "",
		Limit: "1000",
	})
	if err != nil {
		return fmt.Errorf("GuildMembers: %w", err)
	}

	var user *dto.User
	for _, v := range members {
		if v.User.Username == "放空" {
			user = v.User
		}
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// 创建私信会话
	dm, err := c.api.CreateDirectMessage(ctx, &dto.DirectMessageToCreate{
		SourceGuildID: c.guild.ID,
		RecipientID:   user.ID,
	})
	if err != nil {
		return fmt.Errorf("CreateDirectMessage: %w", err)
	}
	fmt.Printf("CreateDirectMessage resp:%+v\n", dm)

	// 发送私信
	reply, err := c.api.PostDirectMessage(ctx, dm, &dto.MessageToCreate{
		Content: content,
	})
	if err != nil {
		return fmt.Errorf("PostDirectMessage err: %w\n", err)
	}
	_ = reply
	return nil
}

// ReadyHandler 自定义 ReadyHandler 感知连接成功事件
func ReadyHandler() event.ReadyHandler {
	return func(event *dto.WSPayload, data *dto.WSReadyData) {
		log.Printf("[ReadyHandler] event: %+v\n data: %+v\n", event, data)
	}
}

func ErrorNotifyHandler() event.ErrorNotifyHandler {
	return func(err error) {
		log.Println("[ErrorNotifyHandler] err: ", err)
	}
}

// PrivateMessage 处理私信事件
func PrivateMessage(api openapi.OpenAPI) event.DirectMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSDirectMessageData) error {
		fmt.Printf("[PrivateMessage] WSDirectMessageData: %+v\n", data)
		// 发送消息回复
		reply, err := api.PostMessage(context.Background(), data.ChannelID, &dto.MessageToCreate{
			Content: fmt.Sprintf("老铁'%s'服务没毛病", data.Author.Username),
		})
		if err != nil {
			return fmt.Errorf("PostDirectMessage err: %w\n", err)
		}
		_ = reply
		return nil
	}
}

// MessageAuditPass 处理审核消息事件,审核消息是腾讯平台审核么？
func MessageAuditPass() event.MessageAuditEventHandler {
	return func(event *dto.WSPayload, data *dto.WSMessageAuditData) error {
		fmt.Printf("[MessageAuditPass] WSDirectMessageData: %+v\n", data)
		return nil
	}
}

// ATMessageEventHandler 实现处理 @ at 消息的回调
func ATMessageEventHandler(api openapi.OpenAPI) event.ATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSATMessageData) error {
		log.Printf("[ATMessageEventHandler] data: %+v\n", data)
		input := strings.ToLower(message.ETLInput(data.Content))
		cmd := message.ParseCommand(input)
		switch cmd.Cmd {
		case "test":
			reply, err := api.PostMessage(context.Background(), data.ChannelID, &dto.MessageToCreate{
				Content: "hello " + data.Author.Username,
			})
			if err != nil {
				log.Printf("PostMessage err1111: %+v\n", err)
				return fmt.Errorf("PostMessage err: %w\n", err)
			}
			_ = reply
			log.Printf("[command.test] %+v\n", data)
		default:
			return fmt.Errorf("ATMessageEventHandler unknown command '%s' data:%+v\n", input, data)
		}
		return nil
	}
}

// CreateMessageHandler 处理消息事件 当发帖子时会触发
func CreateMessageHandler() event.MessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSMessageData) error {
		fmt.Printf("[CreateMessageHandler]: %+v\n", data)
		return nil
	}
}
