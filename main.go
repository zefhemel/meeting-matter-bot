package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mattermost/mattermost-server/model"
)

const helpText string = `
# Meeting Matter Bot

Supported commands:

To the bot account:
* ping: Check if server is up, will respond with checkmark
* help: This message

In a group chat:
* Add the #topic hashtag to a message to add a topics
* "list topics" to list all non completed topics
* "complete all" to mark all topics as complete
`

var userCache map[string]*model.User = map[string]*model.User{}
var channelCache map[string]*model.Channel = map[string]*model.Channel{}
var mmClient *model.Client4
var botUser *model.User

func lookupUser(userID string) *model.User {
	user := userCache[userID]
	if user != nil {
		return user
	} else {
		userCache[userID], _ = mmClient.GetUser(userID, "")
		return userCache[userID]
	}
}

func lookupChannel(channelID string) *model.Channel {
	channel := channelCache[channelID]
	if channel != nil {
		return channel
	} else {
		channelCache[channelID], _ = mmClient.GetChannel(channelID, "")
		return channelCache[channelID]
	}
}

func cleanMessageHashtags(s, hashtags string) string {
	for _, ht := range strings.Split(hashtags, " ") {
		s = strings.ReplaceAll(s, ht, "")
	}
	return s
}

func listTopicPosts(channelID string) ([]*model.Post, error) {
	pl, resp := mmClient.GetPostsForChannel(channelID, 0, 100, "")
	topicPosts := make([]*model.Post, 0, 20)
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Error
	}
topicLoop:
	for _, postID := range pl.Order {
		channelPost := pl.Posts[postID]
		if channelPost.Hashtags == "#topic" {
			if channelPost.UserId == botUser.Id {
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

func handlePosted(evt *model.WebSocketEvent) {
	post := model.Post{}
	err := json.Unmarshal([]byte(evt.Data["post"].(string)), &post)
	if err != nil {
		fmt.Printf("Could not unmarshall post: %v\n", err)
		return
	}
	channel := lookupChannel(post.ChannelId)
	if channel.Type == model.CHANNEL_DIRECT {
		if post.Message == "ping" {
			_, resp := mmClient.SaveReaction(&model.Reaction{
				UserId:    botUser.Id,
				PostId:    post.Id,
				EmojiName: "ping_pong",
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond to ping: %+v", resp)
			}
			return
		}

		if post.Message == "help" {
			_, resp := mmClient.CreatePost(&model.Post{
				ParentId:  post.Id,
				RootId:    post.Id,
				Message:   helpText,
				ChannelId: post.ChannelId,
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond: %+v", resp)
			}
			return
		}
	} else if channel.Type == model.CHANNEL_GROUP {
		if post.Hashtags == "#topic" {
			_, resp := mmClient.SaveReaction(&model.Reaction{
				UserId:    botUser.Id,
				PostId:    post.Id,
				EmojiName: "pencil2",
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond: %+v", resp)
				return
			}
		}
		if post.Message == "list topics" {
			topicPosts, err := listTopicPosts(post.ChannelId)
			if err != nil {
				fmt.Printf("ERror: %+v", err)
				return
			}
			topicMessages := make([]string, 0, 20)
			for _, topicPost := range topicPosts {
				topicMessages = append(topicMessages, fmt.Sprintf("* %s", cleanMessageHashtags(topicPost.Message, topicPost.Hashtags)))
			}
			_, resp := mmClient.CreatePost(&model.Post{
				ParentId:  post.Id,
				RootId:    post.Id,
				Message:   strings.Join(topicMessages, "\n"),
				ChannelId: post.ChannelId,
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond: %+v", resp)
			}
		} else if post.Message == "complete all" {
			topicPosts, err := listTopicPosts(post.ChannelId)
			if err != nil {
				fmt.Printf("ERror: %+v", err)
				return
			}
			for _, topicPost := range topicPosts {
				_, resp := mmClient.SaveReaction(&model.Reaction{
					UserId:    botUser.Id,
					PostId:    topicPost.Id,
					EmojiName: "white_check_mark",
				})
				if resp.StatusCode != 200 {
					fmt.Printf("Failed to respond: %+v", resp)
					return
				}
			}
			_, resp := mmClient.SaveReaction(&model.Reaction{
				UserId:    botUser.Id,
				PostId:    post.Id,
				EmojiName: "white_check_mark",
			})
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to respond: %+v", resp)
				return
			}
		}
	}

	fmt.Printf("Here is the post %+v\n", post)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	mmClient = model.NewAPIv4Client(os.Getenv("server"))
	mmClient.SetOAuthToken(os.Getenv("token"))

	ws, err := model.NewWebSocketClient4(os.Getenv("wsserver"), os.Getenv("token"))
	if err != nil {
		fmt.Printf("Error: %+v", err)
	}
	err = ws.Connect()
	if err != nil {
		fmt.Printf("Error: %+v", err)
	}
	botUser, _ = mmClient.GetMe("")

	ws.Listen()

	for evt := range ws.EventChannel {
		fmt.Printf("Got event: %+v\n", evt)

		if evt.EventType() == "posted" {
			handlePosted(evt)
		}
	}

}
