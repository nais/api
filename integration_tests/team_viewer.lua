Test.gql("Check team is viewer / owner", function(t)
	Helper.SQLExec([[
    	INSERT INTO teams(
    		slug,
    		purpose,
    		slack_channel
    	) VALUES (
    		'member',
    		'Member',
    		'#member'
    	), (
    		'owner',
    		'Owner',
    		'#owner'
    	), (
    		'not-a-member',
    		'not-a-member',
    		'#not-a-member'
    	);
    ]])

	Helper.SQLExec([[
    	INSERT INTO user_roles(
    		role_name,
    		user_id,
    		target_team_slug
    	) VALUES (
    		'Team member',
    		(SELECT id FROM users WHERE email = 'authenticated@example.com'),
    		'member'
    	), (
    		'Team owner',
    		(SELECT id FROM users WHERE email = 'authenticated@example.com'),
    		'owner'
    	);
    ]])

	t.query [[
		query {
			team1: team(slug:"member") {
				viewerIsMember
				viewerIsOwner
			}

			team2: team(slug:"owner") {
				viewerIsMember
				viewerIsOwner
			}

			team3: team(slug:"not-a-member") {
				viewerIsMember
				viewerIsOwner
			}
		}
	]]

	t.check {
		data = {
			team1 = {
				viewerIsMember = true,
				viewerIsOwner = false
			},
			team2 = {
				viewerIsMember = true,
				viewerIsOwner = true
			},
			team3 = {
				viewerIsMember = false,
				viewerIsOwner = false
			}
		}
	}
end)
