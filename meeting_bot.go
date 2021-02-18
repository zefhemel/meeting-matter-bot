package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	_ "embed"

	"github.com/mattermost/mattermost-server/model"
)

//go:embed HELP.md
var helpText string

// MeetingBot represent the meeting bot object
type MeetingBot struct {
	userCache    map[string]*model.User
	channelCache map[string]*model.Channel
	mmClient     *model.Client4
	wsClient     *model.WebSocketClient
	botUser      *model.User
}

// NewMeetingBot creates a new instance of the meeting bot
func NewMeetingBot(url, wsURL, token string) (*MeetingBot, error) {
	mb := &MeetingBot{
		userCache:    map[string]*model.User{},
		channelCache: map[string]*model.Channel{},
		mmClient:     model.NewAPIv4Client(url),
	}
	mb.mmClient.SetOAuthToken(token)

	var err *model.AppError
	mb.wsClient, err = model.NewWebSocketClient4(wsURL, token)
	if err != nil {
		fmt.Println("Err", err)
		return nil, err
	}

	var resp *model.Response
	mb.botUser, resp = mb.mmClient.GetMe("")
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Could not get bot account")
	}

	return mb, nil
}

// Listen listens and handles incoming messages until the server dies
func (mb *MeetingBot) Listen() error {
	err := mb.wsClient.Connect()

	if err != nil {
		return err
	}

	mb.wsClient.Listen()

	for evt := range mb.wsClient.EventChannel {
		fmt.Printf("Got event: %+v\n", evt)

		if evt.EventType() == "posted" {
			mb.handlePosted(evt)
		}
	}
	return nil
}

func (mb *MeetingBot) lookupUser(userID string) *model.User {
	user := mb.userCache[userID]
	if user != nil {
		return user
	} else {
		mb.userCache[userID], _ = mb.mmClient.GetUser(userID, "")
		return mb.userCache[userID]
	}
}

func (mb *MeetingBot) lookupChannel(channelID string) *model.Channel {
	channel := mb.channelCache[channelID]
	if channel != nil {
		return channel
	} else {
		mb.channelCache[channelID], _ = mb.mmClient.GetChannel(channelID, "")
		return mb.channelCache[channelID]
	}
}

func (mb *MeetingBot) listTopicPosts(channelID string) ([]*model.Post, error) {
	pl, resp := mb.mmClient.GetPostsForChannel(channelID, 0, 100, "")
	topicPosts := make([]*model.Post, 0, 20)
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Error
	}
topicLoop:
	for _, postID := range pl.Order {
		channelPost := pl.Posts[postID]
		if channelPost.Hashtags == "#topic" {
			if channelPost.UserId == mb.botUser.Id {
				continue topicLoop
			}
			for _, reaction := range channelPost.Metadata.Reactions {
				if strings.Contains(reaction.EmojiName, "check") {
					continue topicLoop
				}
			}
			topicPosts = append(topicPosts, channelPost)
		}
	}
	return topicPosts, nil
}

func (mb *MeetingBot) handleDirect(post *model.Post, channel *model.Channel) {
	switch post.Message {
	case "ping":
		_, resp := mb.mmClient.SaveReaction(&model.Reaction{
			UserId:    mb.botUser.Id,
			PostId:    post.Id,
			EmojiName: "ping_pong",
		})
		if resp.StatusCode != 200 {
			fmt.Printf("Failed to respond to ping: %+v", resp)
		}
	case "help":
		_, resp := mb.mmClient.CreatePost(&model.Post{
			ParentId:  post.Id,
			RootId:    post.Id,
			Message:   helpText,
			ChannelId: post.ChannelId,
		})
		if resp.StatusCode != http.StatusCreated {
			fmt.Printf("Failed to respond: %+v", resp)
		}
	}
}

func (mb *MeetingBot) handleChannel(post *model.Post, channel *model.Channel) {
	switch post.Hashtags {
	case "#topic", "#agenda":
		_, resp := mb.mmClient.SaveReaction(&model.Reaction{
			UserId:    mb.botUser.Id,
			PostId:    post.Id,
			EmojiName: "pencil2",
		})
		if resp.StatusCode != 200 {
			fmt.Printf("Failed to respond: %+v", resp)
			return
		}
	case "#todo", "#task":
		_, resp := mb.mmClient.SaveReaction(&model.Reaction{
			UserId:    mb.botUser.Id,
			PostId:    post.Id,
			EmojiName: "memo",
		})
		if resp.StatusCode != 200 {
			fmt.Printf("Failed to respond: %+v", resp)
			return
		}

	}
	if strings.Contains(post.Message, fmt.Sprintf("@%s", mb.botUser.Username)) {
		switch cleanMessage(post.Message) {
		case "list topics":
			topicPosts, err := mb.listTopicPosts(post.ChannelId)
			if err != nil {
				fmt.Printf("ERror: %+v", err)
				return
			}
			topicMessages := make([]string, 0, 20)
			for _, topicPost := range topicPosts {
				topicMessages = append(topicMessages, fmt.Sprintf("* %s", cleanMessageHashtags(topicPost.Message)))
			}
			_, resp := mb.mmClient.CreatePost(&model.Post{
				ParentId:  post.Id,
				RootId:    post.Id,
				Message:   strings.Join(topicMessages, "\n"),
				ChannelId: post.ChannelId,
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond: %+v", resp)
			}

		case "complete all":
			topicPosts, err := mb.listTopicPosts(post.ChannelId)
			if err != nil {
				fmt.Printf("ERror: %+v", err)
				return
			}
			for _, topicPost := range topicPosts {
				_, resp := mb.mmClient.SaveReaction(&model.Reaction{
					UserId:    mb.botUser.Id,
					PostId:    topicPost.Id,
					EmojiName: "white_check_mark",
				})
				if resp.StatusCode != 200 {
					fmt.Printf("Failed to respond: %+v", resp)
					return
				}
			}
			_, resp := mb.mmClient.SaveReaction(&model.Reaction{
				UserId:    mb.botUser.Id,
				PostId:    post.Id,
				EmojiName: "white_check_mark",
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond: %+v", resp)
				return
			}
		}
	}
}

func (mb *MeetingBot) handlePosted(evt *model.WebSocketEvent) {
	post := model.Post{}
	err := json.Unmarshal([]byte(evt.Data["post"].(string)), &post)
	if err != nil {
		fmt.Printf("Could not unmarshall post: %v\n", err)
		return
	}
	channel := mb.lookupChannel(post.ChannelId)
	if channel.Type == model.CHANNEL_DIRECT {
		mb.handleDirect(&post, channel)
	} else if channel.Type == model.CHANNEL_GROUP {
		mb.handleChannel(&post, channel)
	}

	fmt.Printf("Here is the post %+v\n", post)
}
