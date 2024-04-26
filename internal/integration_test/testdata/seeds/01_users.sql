-- Create 20 users with random email, name and external_id
INSERT INTO users (
  email,
  name,
  external_id
)
SELECT
  concat('email-', generate_series, '@example.com'),
  concat('name-', generate_series),
  concat('external_id-', generate_series)
FROM generate_series(1, 20);

-- Create one user with a specific email, name and external_id
INSERT INTO users (
  email,
  name,
  external_id
)
VALUES (
  'authenticated@example.com',
  'Authenticated User',
  'authenticated'
);
