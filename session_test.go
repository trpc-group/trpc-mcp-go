package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	// Create session manager
	manager := NewSessionManager(3600) // 1 hour expiry time

	// Verify object created successfully
	assert.NotNil(t, manager)
	assert.Empty(t, manager.getActiveSessions())
}

func TestSessionManager_CreateSession(t *testing.T) {
	// Create session manager
	manager := NewSessionManager(3600)

	// Create session
	session := manager.createSession()

	// Verify session
	assert.NotEmpty(t, session.GetID())
	assert.WithinDuration(t, time.Now(), session.GetCreatedAt(), 1*time.Second)
	assert.WithinDuration(t, time.Now(), session.GetLastActivity(), 1*time.Second)

	// Verify session is stored in the manager
	sessions := manager.getActiveSessions()
	assert.Len(t, sessions, 1)
	assert.Contains(t, sessions, session.GetID())
}

func TestSessionManager_GetSession(t *testing.T) {
	// Create session manager
	manager := NewSessionManager(3600)

	// Test cases
	testCases := []struct {
		name         string
		sessionID    string
		shouldExist  bool
		shouldUpdate bool
	}{
		{
			name:         "Existing session",
			shouldExist:  true,
			shouldUpdate: true,
		},
		{
			name:         "Non-existent session",
			sessionID:    "non-existent-session",
			shouldExist:  false,
			shouldUpdate: false,
		},
		{
			name:         "Empty session ID",
			sessionID:    "",
			shouldExist:  false,
			shouldUpdate: false,
		},
	}

	// Create a session for testing
	existingSession := manager.createSession()

	// Record initial access time
	initialTime := existingSession.GetLastActivity()

	// Wait a short time to ensure timestamp changes can be detected
	time.Sleep(10 * time.Millisecond)

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sessionID := tc.sessionID
			if tc.shouldExist {
				sessionID = existingSession.GetID()
			}

			session, exists := manager.getSession(sessionID)

			if tc.shouldExist {
				assert.True(t, exists)
				assert.NotNil(t, session)
				assert.Equal(t, existingSession.GetID(), session.GetID())
				assert.Equal(t, existingSession.GetCreatedAt(), session.GetCreatedAt())

				if tc.shouldUpdate {
					// Verify LastActivity has been updated
					assert.True(t, session.GetLastActivity().After(initialTime))
				}
			} else {
				assert.False(t, exists)
				assert.Nil(t, session)
			}
		})
	}
}

func TestSessionManager_TerminateSession(t *testing.T) {
	// Create session manager
	manager := NewSessionManager(3600)

	// Create a session
	session := manager.createSession()

	// Verify session exists
	sessions := manager.getActiveSessions()
	assert.Len(t, sessions, 1)
	assert.Contains(t, sessions, session.GetID())

	// Terminate session
	success := manager.terminateSession(session.GetID())

	// Verify session has been terminated
	assert.True(t, success)
	sessions = manager.getActiveSessions()
	assert.Empty(t, sessions)
	assert.NotContains(t, sessions, session.GetID())

	// Terminate non-existent session
	success = manager.terminateSession("non-existent-session")
	assert.False(t, success)
	sessions = manager.getActiveSessions()
	assert.Empty(t, sessions)
}

func TestSession_UpdateActivity(t *testing.T) {
	// Create session
	session := newSession()
	initialTime := session.GetLastActivity()

	// Wait a short time
	time.Sleep(10 * time.Millisecond)

	// Update activity time
	session.UpdateActivity()

	// Verify activity time has been updated
	assert.True(t, session.GetLastActivity().After(initialTime))
}

func TestSession_DataOperations(t *testing.T) {
	// Create session
	session := newSession()

	// Test storing and retrieving data
	key := "testKey"
	value := "testValue"

	// Verify data doesn't exist initially
	_, exists := session.GetData(key)
	assert.False(t, exists)

	// Set data
	session.SetData(key, value)

	// Retrieve data
	retrievedValue, exists := session.GetData(key)
	require.True(t, exists)
	assert.Equal(t, value, retrievedValue)

	// Update data
	newValue := "newValue"
	session.SetData(key, newValue)

	// Retrieve updated data
	retrievedValue, exists = session.GetData(key)
	require.True(t, exists)
	assert.Equal(t, newValue, retrievedValue)
}

func TestSessionContext(t *testing.T) {
	// Create session
	session := newSession()

	// Test storing and retrieving from context
	ctx := setSessionToContext(context.Background(), session)

	// Get from context
	retrievedSession, ok := GetSessionFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, session.GetID(), retrievedSession.GetID())
}
