package bot

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestHandleMyPosts_NoPosts(t *testing.T) {
	conn := newTestDB(t)
	defer func() { _ = conn.Close() }()
	res := HandleMyPosts(conn, 12345, "en")
	if res != "You have no posts." {
		t.Fatalf("unexpected response: %s", res)
	}
}

func TestHandleMyPosts_WithPosts(t *testing.T) {
	conn := newTestDB(t)
	defer func() { _ = conn.Close() }()
	// insert user and posts
	if _, err := conn.Exec("INSERT INTO users (id, username) VALUES (?, ?)", 12345, "tester"); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	if _, err := conn.Exec("INSERT INTO posts (user_id, status, title, created_at) VALUES (?, ?, ?, datetime('now'))", 12345, StatusPending, "Test Title"); err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}
	if _, err := conn.Exec("INSERT INTO posts (user_id, status, title, created_at) VALUES (?, ?, ?, datetime('now'))", 12345, StatusApproved, "Another Title"); err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}
	res := HandleMyPosts(conn, 12345, "en")
	if res == "You have no posts." || res == "" {
		t.Fatalf("unexpected empty response: %s", res)
	}
}

func TestDeleteAndMarkSold(t *testing.T) {
	conn := newTestDB(t)
	defer func() { _ = conn.Close() }()
	// insert user and a post
	if _, err := conn.Exec("INSERT INTO users (id, username) VALUES (?, ?)", 9999, "owner"); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	res, err := conn.Exec("INSERT INTO posts (user_id, status, title, created_at) VALUES (?, ?, ?, datetime('now'))", 9999, StatusPending, "ToDelete")
	if err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}
	postID, _ := res.LastInsertId()

	// mark sold should succeed
	ok, msg := markPostSold(conn, 9999, postID, "en")
	if !ok {
		t.Fatalf("markPostSold failed: %s", msg)
	}

	// delete by owner should succeed
	ok2, msg2 := deletePost(conn, 9999, postID, "en")
	if !ok2 {
		t.Fatalf("deletePost failed: %s", msg2)
	}
}
