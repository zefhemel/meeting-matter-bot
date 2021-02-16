package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/mattermost/mattermost-server/model"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	mmClient := model.NewAPIv4Client(os.Getenv("server"))
	mmClient.SetOAuthToken(os.Getenv("token"))
	posts, _ := mmClient.GetPostsForChannel(os.Getenv("test-channel"), 0, 20, "")
	for _, postID := range posts.Order {
		post := posts.Posts[postID]
		user, _ := mmClient.GetUser(post.UserId, "")
		fmt.Printf("[%s] %s\n", user.Username, post.Message)
	}
}
