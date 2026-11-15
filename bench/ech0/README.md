# Ech0, GHSA-8mc6-xjpr-h98x (SSRF), commit cecc2c19 (the parent of the fix 091d26d2)

Not scored: **a second-order flow**. An administrator adds a peer URL (`AddConnect`), it is stored in
the database, and `fetchPeerConnectInfo` later requests it. rivus has no model of what a database
holds, so the flow from the request to `httpUtil.SendRequest` is not found (and `SendRequest` is a
project function: it needs a project rule for the sink). Recorded as a limit of what rivus can see,
not as a miss in the scoring.
