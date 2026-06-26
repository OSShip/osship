package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type githubClient struct {
	token string
	http  *http.Client
}

func newGitHubClient(token string) *githubClient {
	return &githubClient{token: token, http: http.DefaultClient}
}

func (c *githubClient) fetchContributions(username string) json.RawMessage {
	profile := c.get("https://api.github.com/users/" + url.PathEscape(username))
	repos := c.get("https://api.github.com/users/" + url.PathEscape(username) + "/repos?sort=updated&per_page=5")
	prs := c.get("https://api.github.com/search/issues?q=" + url.QueryEscape("author:"+username+" type:pr") + "&sort=updated&per_page=10")
	events := c.get("https://api.github.com/users/" + url.PathEscape(username) + "/events/public?per_page=10")

	result := map[string]json.RawMessage{
		"profile": profile,
		"repos":   repos,
		"pull_requests": prs,
		"recent_events": events,
		"summary":       buildSummary(profile, prs, repos),
	}
	data, _ := json.Marshal(result)
	return data
}

func (c *githubClient) get(rawURL string) json.RawMessage {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return json.RawMessage(`{"error":"request failed"}`)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return json.RawMessage(`{"error":"fetch failed"}`)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return json.RawMessage(`{"error":"github api error","status":` + jsonNumber(resp.StatusCode) + `}`)
	}
	return body
}

func buildSummary(profile, prs, repos json.RawMessage) json.RawMessage {
	summary := map[string]interface{}{
		"public_repos": 0,
		"followers":    0,
		"open_prs":     0,
		"total_prs":    0,
	}
	var p struct {
		PublicRepos int `json:"public_repos"`
		Followers   int `json:"followers"`
		Login       string `json:"login"`
	}
	if json.Unmarshal(profile, &p) == nil {
		summary["public_repos"] = p.PublicRepos
		summary["followers"] = p.Followers
		summary["login"] = p.Login
	}
	var prSearch struct {
		TotalCount int `json:"total_count"`
	}
	if json.Unmarshal(prs, &prSearch) == nil {
		summary["total_prs"] = prSearch.TotalCount
	}
	var repoList []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(repos, &repoList) == nil && len(repoList) > 0 {
		names := make([]string, 0, len(repoList))
		for _, r := range repoList {
			names = append(names, r.Name)
		}
		summary["recent_repos"] = names
	}
	data, _ := json.Marshal(summary)
	return data
}

func jsonNumber(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
