// Cleanup graphql statuses in case of the

const statusContext = "GraphQL Review";
const statusDescription = "No GraphQL changes detected";

module.exports = async ({ github, context }) => {
	// Check if a commit status already exists for GraphQL review
	const list = await github.rest.repos.listCommitStatusesForRef({
		owner: context.repo.owner,
		repo: context.repo.repo,
		ref: context.payload.pull_request.head.sha,
	});

	// Remove any existing GraphQL review statuses and labels
	await github.rest.repos.createCommitStatus({
		owner: context.repo.owner,
		repo: context.repo.repo,
		sha: context.payload.pull_request.head.sha,
		state: "success",
		context: statusContext,
		description: statusDescription,
	});

	const { data: labels } = await github.rest.issues.listLabelsOnIssue({
		owner: context.repo.owner,
		repo: context.repo.repo,
		issue_number: prNumber,
	});
	const hasLabel = labels.some((label) => label.name === "graphql-review-required");
	// Remove label if it exists
	if (hasLabel) {
		await github.rest.issues.removeLabel({
			owner: context.repo.owner,
			repo: context.repo.repo,
			issue_number: context.issue.number,
			name: "graphql-review-required",
		});
	}
};
