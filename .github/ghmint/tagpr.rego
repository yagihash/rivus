package ghmint

issuer := "https://token.actions.githubusercontent.com"

permissions := {"contents": "write", "pull_requests": "write"}

allow if {
    input.sub == "repo:yagihash@1553908/rivus@1360015006:environment:release"
    input.ref == "refs/heads/main"
    input.event_name == "push"
    input.job_workflow_ref == "yagihash/rivus/.github/workflows/tagpr.yml@refs/heads/main"
}
