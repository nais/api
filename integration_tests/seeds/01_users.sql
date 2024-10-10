-- Create 20 users with random email, name and external_id
INSERT INTO
	users (email, name, external_id)
SELECT
	CONCAT('email-', GENERATE_SERIES, '@example.com'),
	CONCAT('name-', GENERATE_SERIES),
	CONCAT('external_id-', GENERATE_SERIES)
FROM
	GENERATE_SERIES(1, 20)
;

-- Create one user with a specific email, name and external_id
INSERT INTO
	users (email, name, external_id)
VALUES
	(
		'authenticated@example.com',
		'Authenticated User',
		'authenticated'
	)
;
