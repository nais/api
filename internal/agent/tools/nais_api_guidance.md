## Nais API Guidance

The Nais API is a GraphQL API for managing applications and jobs on the Nais platform.

### Key Concepts

- **Team**: The primary organizational unit. All resources belong to a team.
- **Application**: A long-running workload (deployment) managed by Nais (Often called only App/app).
- **Job**: A scheduled or one-off workload (CronJob/Job) managed by Nais.
- **Environment**: A Kubernetes cluster/namespace where workloads run (e.g., "dev", "prod").
- **Workload**: A union type representing either an Application or Job.

### Common Query Patterns

1. **Get current user and their teams**:
   ```graphql
   query { me { ... on User { teams(first: 50) { nodes { team { slug } } } } } }
   ```

2. **Get team details**:
   ```graphql
   query($slug: Slug!) { team(slug: $slug) { slug purpose slackChannel } }
   ```

3. **List applications for a team**:
   ```graphql
   query($slug: Slug!) {
     team(slug: $slug) {
       applications(first: 50) {
         nodes { name state teamEnvironment { environment { name } } }
       }
     }
   }
   ```

4. **Get application details with instances**:
   ```graphql
   query($slug: Slug!, $name: String!, $env: [String!]) {
     team(slug: $slug) {
       applications(filter: { name: $name, environments: $env }, first: 1) {
         nodes {
           name state
           instances { nodes { name restarts status { state message } } }
           image { name tag }
         }
       }
     }
   }
   ```

5. **List jobs for a team**:
   ```graphql
   query($slug: Slug!) {
     team(slug: $slug) {
       jobs(first: 50) {
         nodes { name state schedule { expression } teamEnvironment { environment { name } } }
       }
     }
   }
   ```

6. **Get vulnerabilities for a workload**:
   ```graphql
   query($slug: Slug!, $name: String!, $env: [String!]) {
     team(slug: $slug) {
       applications(filter: { name: $name, environments: $env }, first: 1) {
         nodes {
           image {
             vulnerabilitySummary { critical high medium low }
             vulnerabilities(first: 20) { nodes { identifier severity package } }
           }
         }
       }
     }
   }
   ```

7. **Get cost information**:
   ```graphql
   query($slug: Slug!) {
     team(slug: $slug) {
       cost { monthlySummary { sum } }
       environments {
         environment { name }
         cost { daily(from: "2024-01-01", to: "2024-01-31") { sum } }
       }
     }
   }
   ```

8. **Search across resources**:
   ```graphql
   query($query: String!) {
     search(filter: { query: $query }, first: 20) {
       nodes {
         __typename
         ... on Application { name team { slug } }
         ... on Job { name team { slug } }
         ... on Team { slug }
       }
     }
   }
   ```

9. **Get alerts for a team**:
   ```graphql
   query($slug: Slug!) {
     team(slug: $slug) {
       alerts(first: 50) {
         nodes { name state teamEnvironment { environment { name } } }
       }
     }
   }
   ```

10. **Get deployments**:
    ```graphql
    query($slug: Slug!) {
      team(slug: $slug) {
        deployments(first: 20) {
          nodes { createdAt repository commitSha statuses { nodes { state } } }
        }
      }
    }
    ```

### Important Types

- **Slug**: A string identifier for teams (e.g., "my-team")
- **Cursor**: Used for pagination (pass to "after" argument)
- **Date**: Format "YYYY-MM-DD" for cost queries
- **ApplicationState**: RUNNING, NOT_RUNNING, UNKNOWN
- **JobState**: RUNNING, NOT_RUNNING, UNKNOWN
- **Severity**: CRITICAL, HIGH, MEDIUM, LOW, UNASSIGNED (for issues/vulnerabilities)

### Pagination

Most list fields support pagination with:
- `first: Int` - Number of items to fetch
- `after: Cursor` - Cursor from previous page
- `pageInfo { hasNextPage endCursor totalCount }` - Pagination info

### Filtering

Many fields support filters:
- `filter: { name: String, environments: [String!] }` - For applications/jobs
- `filter: { severity: Severity }` - For issues

### Tips

1. DO NOT query secret-related types/fields (Secret, SecretValue, etc.)
2. Always use `__typename` when querying union/interface types (Workload, Issue, etc.)
3. Use fragment spreads for type-specific fields: `... on Application { ingresses { url } }`
4. Start with schema exploration to discover available fields
5. Use pagination for large result sets (default to first: 50)

### Nais Console URLs

When providing links to the user, use the console URL patterns provided by the `get_nais_context` tool.
Call `get_nais_context` to get the base URL and all available URL patterns with placeholders.
Replace the placeholders (e.g., `{team}`, `{env}`, `{app}`) with actual values from query results.

**Note**: Do NOT invent or guess URLs. Only use the URL patterns from `get_nais_context` with actual data from query results.