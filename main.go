// Command txid is a CLI for interacting with api.txid.uk.
// Uses Bearer token authentication (create via admin page or /auth/tokens).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAPI = "https://api.txid.uk"
	version    = "0.1.0"
)

var (
	apiURL    = defaultAPI
	tokenPath string
)

func main() {
	// Allow overriding API via env
	if v := os.Getenv("TXID_API"); v != "" {
		apiURL = v
	}

	home, err := os.UserHomeDir()
	if err != nil {
		die("cannot determine home: %v", err)
	}
	tokenPath = filepath.Join(home, ".config", "txid", "token")

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "auth":
		cmdAuth(args)
	case "whoami":
		cmdWhoami()
	case "logout":
		cmdLogout()
	case "search":
		cmdSearch(args)
	case "notif", "notifications":
		cmdNotif(args)
	case "sub", "subscribe":
		cmdSub(args)
	case "channels":
		cmdChannels()
	case "logs":
		cmdLogs(args)
	case "stats":
		cmdStats()
	case "open":
		cmdOpen(args)
	case "token", "tokens":
		cmdToken(args)
	case "lib":
		cmdLib(args)
	case "push":
		cmdPush(args)
	case "version", "-v", "--version":
		fmt.Printf("txid-cli %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`txid - CLI for api.txid.uk

Usage:
  txid <command> [args...]

Commands:
  auth <token>        Save API Bearer token (format: txid_...)
  whoami              Show current authenticated user
  logout              Delete saved token
  search <query>      Unified search across posts/glossary/users
  notif [--unread]    List recent notifications
  sub                 List channel subscriptions
  sub <channel>       Toggle subscription for a channel
  channels            List all available notification channels
  token list          List your API tokens
  token create <name> Create a new API token
  logs [-s src] [-f]  View recent logs (-f to follow)
  stats               Hub stats (channels, counts, last push)
  open <subdomain>    Open subdomain in browser (e.g. txid open dash)
  lib status          Show your lib.txid.uk reading progress
  push <ch> <title> [body]
                      Push a notification to a channel
                      (requires TXID_NOTIFICATION_SECRET in env)
  version             Show CLI version
  help                Show this message

Environment:
  TXID_API            Override API URL (default: https://api.txid.uk)

Auth:
  Create a token at https://api.txid.uk/admin.html (OAuth Clients tab
  or use /auth/tokens endpoint), then run: txid auth txid_xxx...`)
}

// ─── Token Storage ───

func loadToken() string {
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveToken(token string) error {
	dir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(tokenPath, []byte(token), 0600)
}

// ─── HTTP Helper ───

func apiRequest(method, path string, body any) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, apiURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	token := loadToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if len(respBody) > 0 && (respBody[0] == '{' || respBody[0] == '[') {
		_ = json.Unmarshal(respBody, &result)
	}

	if resp.StatusCode >= 400 {
		errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if result != nil {
			if e, ok := result["error"].(string); ok {
				errMsg += ": " + e
			}
		}
		return result, fmt.Errorf("%s", errMsg)
	}
	return result, nil
}

func apiRequestList(method, path string) ([]any, error) {
	req, err := http.NewRequest(method, apiURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := loadToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, respBody)
	}

	var list []any
	if err := json.Unmarshal(respBody, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// ─── Commands ───

func cmdAuth(args []string) {
	if len(args) == 0 {
		die("usage: txid auth <token>\nCreate a token at https://api.txid.uk/admin.html")
	}
	token := args[0]
	if !strings.HasPrefix(token, "txid_") {
		die("invalid token format (must start with txid_)")
	}
	if err := saveToken(token); err != nil {
		die("save token: %v", err)
	}
	fmt.Printf("✓ Token saved to %s\n", tokenPath)

	// Verify
	res, err := apiRequest("GET", "/auth/me", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: token verification failed: %v\n", err)
		return
	}
	if auth, _ := res["authenticated"].(bool); auth {
		pk, _ := res["pubkey"].(string)
		fmt.Printf("✓ Authenticated as %s\n", shortPubkey(pk))
	}
}

func cmdWhoami() {
	if loadToken() == "" {
		die("not authenticated. run: txid auth <token>")
	}
	res, err := apiRequest("GET", "/auth/me", nil)
	if err != nil {
		die("%v", err)
	}
	if auth, _ := res["authenticated"].(bool); !auth {
		die("token invalid or expired")
	}
	pk, _ := res["pubkey"].(string)
	name, _ := res["displayName"].(string)
	if name == "" {
		name = "(no name)"
	}
	fmt.Printf("Pubkey:   %s\n", pk)
	fmt.Printf("Name:     %s\n", name)
	if np, _ := res["nostrPubkey"].(string); np != "" {
		fmt.Printf("Nostr:    %s\n", np)
	}
	if pts, ok := res["points"].(float64); ok {
		fmt.Printf("Points:   %.0f\n", pts)
	}
}

func cmdLogout() {
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		die("remove token: %v", err)
	}
	fmt.Println("✓ Logged out")
}

func cmdSearch(args []string) {
	if len(args) == 0 {
		die("usage: txid search <query>")
	}
	q := strings.Join(args, " ")
	res, err := apiRequest("GET", "/search?q="+encodeQuery(q)+"&limit=5", nil)
	if err != nil {
		die("%v", err)
	}

	total, _ := res["total"].(float64)
	if total == 0 {
		fmt.Println("No results")
		return
	}

	results, _ := res["results"].(map[string]any)
	for _, typ := range []string{"posts", "glossary", "users"} {
		items, _ := results[typ].([]any)
		if len(items) == 0 {
			continue
		}
		fmt.Printf("\n── %s (%d) ──\n", strings.ToUpper(typ), len(items))
		for _, it := range items {
			m, _ := it.(map[string]any)
			title := cleanMark(str(m, "title"))
			snippet := cleanMark(str(m, "snippet"))
			url := str(m, "url")
			fmt.Printf("  %s\n", title)
			if snippet != "" {
				if len(snippet) > 100 {
					snippet = snippet[:100] + "..."
				}
				fmt.Printf("    %s\n", snippet)
			}
			fmt.Printf("    %s\n", url)
		}
	}
}

func cmdNotif(args []string) {
	unreadOnly := false
	for _, a := range args {
		if a == "--unread" {
			unreadOnly = true
		}
	}
	res, err := apiRequest("GET", "/notifications?limit=20", nil)
	if err != nil {
		die("%v", err)
	}
	unread, _ := res["unreadCount"].(float64)
	items, _ := res["items"].([]any)

	fmt.Printf("Unread: %d\n\n", int(unread))
	count := 0
	for _, it := range items {
		m, _ := it.(map[string]any)
		isRead, _ := m["isRead"].(bool)
		if unreadOnly && isRead {
			continue
		}
		marker := "○"
		if isRead {
			marker = "·"
		}
		ts, _ := m["createdAt"].(float64)
		channelID, _ := m["channelId"].(string)
		title, _ := m["title"].(string)
		body, _ := m["body"].(string)
		fmt.Printf("%s [%s] %s\n", marker, channelID, title)
		if body != "" {
			fmt.Printf("  %s\n", body)
		}
		fmt.Printf("  %s\n\n", time.Unix(int64(ts), 0).Format("01-02 15:04"))
		count++
	}
	if count == 0 {
		fmt.Println("(no notifications)")
	}
}

func cmdSub(args []string) {
	if len(args) == 0 {
		// List subscriptions
		items, err := apiRequestList("GET", "/notifications/subscriptions")
		if err != nil {
			die("%v", err)
		}
		for _, it := range items {
			m, _ := it.(map[string]any)
			id := str(m, "id")
			name := str(m, "name")
			enabled, _ := m["enabled"].(bool)
			marker := "[ ]"
			if enabled {
				marker = "[✓]"
			}
			fmt.Printf("%s %s (%s)\n", marker, name, id)
		}
		return
	}
	// Toggle: get current state first
	channel := args[0]
	items, err := apiRequestList("GET", "/notifications/subscriptions")
	if err != nil {
		die("%v", err)
	}
	current := false
	for _, it := range items {
		m, _ := it.(map[string]any)
		if str(m, "id") == channel {
			current, _ = m["enabled"].(bool)
			break
		}
	}
	// Toggle
	_, err = apiRequest("PUT", "/notifications/subscriptions/"+channel, map[string]any{"enabled": !current})
	if err != nil {
		die("%v", err)
	}
	if !current {
		fmt.Printf("✓ Subscribed to %s\n", channel)
	} else {
		fmt.Printf("✓ Unsubscribed from %s\n", channel)
	}
}

func cmdChannels() {
	items, err := apiRequestList("GET", "/notifications/channels")
	if err != nil {
		die("%v", err)
	}
	fmt.Printf("%-16s %-20s %s\n", "ID", "NAME", "DESCRIPTION")
	fmt.Printf("%-16s %-20s %s\n", "──", "────", "───────────")
	for _, it := range items {
		m, _ := it.(map[string]any)
		fmt.Printf("%-16s %-20s %s\n", str(m, "id"), str(m, "name"), str(m, "description"))
	}
}

func cmdToken(args []string) {
	if len(args) == 0 {
		die("usage: txid token list | txid token create <name>")
	}
	switch args[0] {
	case "list":
		items, err := apiRequestList("GET", "/auth/tokens")
		if err != nil {
			die("%v", err)
		}
		fmt.Printf("%-4s %-20s %s\n", "ID", "LABEL", "SCOPES")
		for _, it := range items {
			m, _ := it.(map[string]any)
			id, _ := m["id"].(float64)
			label := str(m, "label")
			scopes := str(m, "scopes")
			fmt.Printf("%-4d %-20s %s\n", int(id), label, scopes)
		}
	case "create":
		if len(args) < 2 {
			die("usage: txid token create <name>")
		}
		res, err := apiRequest("POST", "/auth/tokens", map[string]any{"label": args[1]})
		if err != nil {
			die("%v", err)
		}
		fmt.Printf("✓ Created token: %s\n", str(res, "token"))
		fmt.Println("⚠️  Save this token now - it will not be shown again")
	default:
		die("unknown token subcommand: %s", args[0])
	}
}

func cmdLogs(args []string) {
	source := ""
	follow := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-s", "--source":
			if i+1 < len(args) {
				source = args[i+1]
				i++
			}
		case "-f", "--follow":
			follow = true
		}
	}

	printLogs := func(seenIDs map[float64]bool) {
		path := "/logs?limit=50"
		if source != "" {
			path += "&source=" + source
		}
		res, err := apiRequest("GET", path, nil)
		if err != nil {
			die("%v", err)
		}
		logs, _ := res["logs"].([]any)
		// Reverse (newest last) for tail-like display
		for i := len(logs) - 1; i >= 0; i-- {
			l, _ := logs[i].(map[string]any)
			id, _ := l["id"].(float64)
			if seenIDs != nil {
				if seenIDs[id] {
					continue
				}
				seenIDs[id] = true
			}
			ts, _ := l["createdAt"].(float64)
			src := str(l, "source")
			level := str(l, "level")
			msg := str(l, "message")
			details := str(l, "details")
			color := levelColor(level)
			fmt.Printf("%s%s\033[0m %-24s %s%-5s\033[0m %s",
				"\033[90m", time.Unix(int64(ts), 0).Format("01-02 15:04:05"),
				src, color, level, msg)
			if details != "" {
				fmt.Printf(" \033[90m(%s)\033[0m", details)
			}
			fmt.Println()
		}
	}

	if !follow {
		printLogs(nil)
		return
	}

	seen := make(map[float64]bool)
	printLogs(seen)
	for {
		time.Sleep(3 * time.Second)
		printLogs(seen)
	}
}

func cmdStats() {
	res, err := apiRequestList("GET", "/notifications/stats/public")
	if err != nil {
		die("%v", err)
	}
	fmt.Printf("%-18s %-20s %-6s %-6s %-8s %s\n", "CHANNEL", "NAME", "24h", "7d", "TOTAL", "LAST PUSH")
	now := time.Now().Unix()
	for _, it := range res {
		m, _ := it.(map[string]any)
		id := str(m, "id")
		name := str(m, "name")
		if len(name) > 18 {
			name = name[:18]
		}
		c24, _ := m["count24h"].(float64)
		c7d, _ := m["count7d"].(float64)
		total, _ := m["totalCount"].(float64)
		lastPush, _ := m["lastPush"].(float64)
		age := "never"
		if lastPush > 0 {
			sec := now - int64(lastPush)
			if sec < 3600 {
				age = fmt.Sprintf("%dm ago", sec/60)
			} else if sec < 86400 {
				age = fmt.Sprintf("%dh ago", sec/3600)
			} else {
				age = fmt.Sprintf("%dd ago", sec/86400)
			}
		}
		fmt.Printf("%-18s %-20s %-6d %-6d %-8d %s\n", id, name, int(c24), int(c7d), int(total), age)
	}
}

func cmdLib(args []string) {
	if len(args) == 0 || args[0] != "status" {
		die("usage: txid lib status")
	}
	res, err := apiRequest("GET", "/progress/lib", nil)
	if err != nil {
		die("%v", err)
	}
	progress, _ := res["progress"].(map[string]any)
	updatedAt, _ := res["updatedAt"].(float64)
	if len(progress) == 0 {
		fmt.Println("No reading progress yet.")
		fmt.Println("Visit https://lib.txid.uk and start reading.")
		return
	}

	// Resolve book ids → titles in one batched call
	ids := make([]string, 0, len(progress))
	for id := range progress {
		ids = append(ids, id)
	}
	books, _ := apiRequestList("GET", "/books/by-ids?ids="+strings.Join(ids, ","))
	titles := map[string]string{}
	for _, b := range books {
		bm, _ := b.(map[string]any)
		bid := fmt.Sprintf("%d", int(numFloat(bm, "id")))
		titles[bid] = str(bm, "title")
	}

	fmt.Printf("%-6s %-7s %s\n", "BOOK", "PROG", "TITLE")
	for id, p := range progress {
		pm, _ := p.(map[string]any)
		pct, _ := pm["percentage"].(float64)
		title := titles[id]
		if title == "" {
			title = "(book #" + id + ")"
		}
		if len(title) > 64 {
			title = title[:61] + "..."
		}
		fmt.Printf("%-6s %3d%%    %s\n", id, int(pct), title)
	}
	if updatedAt > 0 {
		t := time.Unix(int64(updatedAt/1000), 0)
		fmt.Printf("\nLast updated: %s\n", t.Format("2006-01-02 15:04 MST"))
	}
}

func cmdPush(args []string) {
	if len(args) < 2 {
		die("usage: txid push <channel> <title> [body]")
	}
	secret := os.Getenv("TXID_NOTIFICATION_SECRET")
	if secret == "" {
		die("TXID_NOTIFICATION_SECRET env var is required for push")
	}
	channelID := args[0]
	title := args[1]
	body := ""
	if len(args) >= 3 {
		body = strings.Join(args[2:], " ")
	}

	payload := map[string]any{"channelId": channelID, "title": title}
	if body != "" {
		payload["body"] = body
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL+"/notifications/push", bytes.NewReader(data))
	if err != nil {
		die("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Notification-Secret", secret)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		die("request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		die("HTTP %d: %s", resp.StatusCode, respBody)
	}
	fmt.Printf("✓ pushed to %s\n", channelID)
}

func cmdOpen(args []string) {
	if len(args) == 0 {
		die("usage: txid open <subdomain>")
	}
	sub := args[0]
	url := "https://" + sub + ".txid.uk"
	if sub == "txid" || sub == "main" {
		url = "https://txid.uk"
	}
	fmt.Println("Opening", url)
	openers := [][]string{
		{"xdg-open", url},
		{"open", url},
		{"cmd", "/c", "start", url},
	}
	for _, cmd := range openers {
		if err := exec.Command(cmd[0], cmd[1:]...).Start(); err == nil {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Failed to open browser. Visit: %s\n", url)
}

func levelColor(level string) string {
	switch level {
	case "debug":
		return "\033[90m"
	case "info":
		return "\033[32m"
	case "warn":
		return "\033[33m"
	case "error":
		return "\033[31m"
	case "fatal":
		return "\033[1;31m"
	}
	return "\033[0m"
}

// ─── Helpers ───

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "txid: "+format+"\n", args...)
	os.Exit(1)
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func numFloat(m map[string]any, key string) float64 {
	f, _ := m[key].(float64)
	return f
}

func shortPubkey(pk string) string {
	if len(pk) < 16 {
		return pk
	}
	return pk[:8] + "..." + pk[len(pk)-4:]
}

func cleanMark(s string) string {
	s = strings.ReplaceAll(s, "<mark>", "")
	s = strings.ReplaceAll(s, "</mark>", "")
	return s
}

func encodeQuery(s string) string {
	// Simple URL encoding for query params
	var out strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '~' {
			out.WriteRune(r)
		} else if r == ' ' {
			out.WriteString("+")
		} else {
			out.WriteString(fmt.Sprintf("%%%02X", r))
		}
	}
	return out.String()
}
