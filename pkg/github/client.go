package github

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
	"k8s.io/klog/v2"
)

// Client wraps GitHub API operations
type Client struct {
	Client *github.Client
}

// NewClient creates a new GitHub client
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return &Client{
		Client: github.NewClient(tc),
	}
}

// GetPendingWorkflows retrieves pending workflow runs
func (c *Client) GetPendingWorkflows(ctx context.Context, owner, repo string) (int, error) {
	// If repo is specified, just query that repository
	if repo != "" {
		return c.getPendingWorkflowsForRepo(ctx, owner, repo)
	}

	// Otherwise, query all repos in the organization
	return c.getPendingWorkflowsForOrg(ctx, owner)
}

// getPendingWorkflowsForRepo gets pending workflows for a specific repository
func (c *Client) getPendingWorkflowsForRepo(ctx context.Context, owner, repo string) (int, error) {
	opts := &github.ListWorkflowRunsOptions{
		Status: "queued",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var totalPending int
	page := 1

	for {
		opts.ListOptions.Page = page
		runs, resp, err := c.Client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
		if err != nil {
			return 0, fmt.Errorf("failed to list workflow runs for %s/%s: %w", owner, repo, err)
		}

		// Count runs that need a self-hosted runner
		for _, run := range runs.WorkflowRuns {
			if run.GetStatus() == "queued" {
				totalPending++
			}
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return totalPending, nil
}

// getPendingWorkflowsForOrg gets pending workflows across all repos in an organization
func (c *Client) getPendingWorkflowsForOrg(ctx context.Context, org string) (int, error) {
	// First, list repositories in the organization
	var allRepos []*github.Repository
	repoOpts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	page := 1
	for {
		repoOpts.Page = page
		repos, resp, err := c.Client.Repositories.ListByOrg(ctx, org, repoOpts)
		if err != nil {
			return 0, fmt.Errorf("failed to list repositories for org %s: %w", org, err)
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	// For large organizations, this could be a lot of API calls
	// Consider implementing caching or other optimizations

	// Query each repository for pending workflow runs
	var totalPending int
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid API rate limits
	semaphore := make(chan struct{}, 5)

	for _, repo := range allRepos {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(repoName string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			pending, err := c.getPendingWorkflowsForRepo(ctx, org, repoName)
			if err != nil {
				klog.Warningf("Error getting pending workflows for %s/%s: %v", org, repoName, err)
				return
			}

			mutex.Lock()
			totalPending += pending
			mutex.Unlock()
		}(repo.GetName())
	}

	wg.Wait()
	return totalPending, nil
}
