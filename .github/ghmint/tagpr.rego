package ghmint

issuer := "https://token.actions.githubusercontent.com"

permissions := {"contents": "write", "pull_requests": "write"}

allow if {
    input.repository == "yagihash/rivus"
    input.ref == "refs/heads/main"
    input.event_name == "push"
    input.job_workflow_ref == "yagihash/rivus/.github/workflows/tagpr.yml@refs/heads/main"
}
