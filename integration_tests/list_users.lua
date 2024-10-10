Test.gql("list users", function(t)
  t.query([[
    {
      users(first: 5) {
        nodes {
          name
          email
        }
        pageInfo {
          totalCount
          endCursor
          hasNextPage
          hasPreviousPage
        }
      }
    }
  ]])

  t.check {
    data = {
      users = {
        nodes = {
          {
            email = "authenticated@example.com",
            name = "Authenticated User"
          },
          {
            email = "email-1@example.com",
            name = "name-1"
          },
          {
            email = "email-10@example.com",
            name = "name-10"
          },
          {
            email = "email-11@example.com",
            name = "name-11"
          },
          {
            email = "email-12@example.com",
            name = "name-12"
          }
        },
        pageInfo = {
          totalCount = 21,
          endCursor = Save("nextPageCursor"),
          hasNextPage = true,
          hasPreviousPage = false
        }
      }
    }
  }
end)

--[[
{
  users(first: 5) {
    nodes {
      name
      email
    }
    pageInfo {
      totalCount
      endCursor
      hasNextPage
      hasPreviousPage
    }
  }
}

RETURNS

OPTION data.users.pageInfo.endCursor=NOTNULL

ENDOPTS

{
  "data": {
    "users": {
      "nodes": [
        {
          "email": "authenticated@example.com",
          "name": "Authenticated User"
        },
        {
          "email": "email-1@example.com",
          "name": "name-1"
        },
        {
          "email": "email-10@example.com",
          "name": "name-10"
        },
        {
          "email": "email-11@example.com",
          "name": "name-11"
        },
        {
          "email": "email-12@example.com",
          "name": "name-12"
        }
      ],
      "pageInfo": {
        "totalCount": 21,
        "endCursor": "something",
        "hasNextPage": true,
        "hasPreviousPage": false
      }
    }
  }
}

STORE nextPageCursor=.data.users.pageInfo.endCursor

]]
