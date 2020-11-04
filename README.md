# Genoa-webhook
A component of Genoa that parses git webhooks and ensures Release CR state matches git state.

1. Parses webhook events from git ( github.com, github-enterprise, gitlab.com, self-hosted gitlab)
2. Applies `Release` CR state from Git into a cluster