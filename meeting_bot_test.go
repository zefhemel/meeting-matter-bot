package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanMessage(t *testing.T) {
	assert.Equal(t, cleanMessage("Hello"), "Hello", "Cleaning without hashtags and mentions should do nothing")

	// Mentions
	assert.Equal(t, cleanMessage("@test Hello @test"), "Hello", "Should clean all mentions")

	// Hashtags
	assert.Equal(t, cleanMessage("Hello #topic"), "Hello", "Should clean hashtags")
	assert.Equal(t, cleanMessage("Hello #topic #topic-2"), "Hello", "Should clean all hashtags")
}
