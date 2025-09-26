module.exports = async ({ github, context }) => {
	const prNumber = context.payload.pull_request.number;
	const { data: reviews } = await github.rest.pulls.listReviews({
		owner: context.repo.owner,
		repo: context.repo.repo,
		pull_number: prNumber,
	});

	// Fetch member logins of the @nais/tooling team
	const { data: teamMembers } = await github.teams.listMembersInOrg({
		org: context.repo.owner,
		team_slug: "tooling",
	});
	const toolingMembers = teamMembers.map((member) => member.login);

	const approvedByTooling = reviews.some(
		(review) => review.state === "APPROVED" && toolingMembers.includes(review.user.login),
	);

	if (!approvedByTooling) {
		// Add label
		await github.issues.addLabels({
			owner: context.repo.owner,
			repo: context.repo.repo,
			issue_number: prNumber,
			labels: ["graphql-review-required"],
		});
		throw new Error(
			"Pull request contains changes to GraphQL schema files but has not been approved by a member of the @nais/tooling team.",
		);
	}

	// Remove label if it exists
	const { data: labels } = await github.issues.listLabelsOnIssue({
		owner: context.repo.owner,
		repo: context.repo.repo,
		issue_number: prNumber,
	});
	const hasLabel = labels.some((label) => label.name === "graphql-review-required");
	if (hasLabel) {
		await github.issues.removeLabel({
			owner: context.repo.owner,
			repo: context.repo.repo,
			issue_number: prNumber,
			name: "graphql-review-required",
		});
	}
};
