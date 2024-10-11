-- This file is generated. Do not edit.


--- Ignore the field, if notNull is true, it will ensure the field is not null
---@param notNull? boolean
---@return string
function Ignore(notNull)
  print("Ignore: ", notNull)
  return ""
end

--- Save the field to the state. By default it will error if the field is null
---@param name string Name of the field in the state
---@param ignoreNull? boolean
---@return string
function Save(name, ignoreNull)
  print("Save: ", name, ignoreNull)
  return ""
end

--- State variables
---@type table<string, any>
State = {}

--- Null ensures the value is null
---@type userdata
Null = {}

---@class TestFunctionTgql
local TestFunctionTgql = {}

--- Query comment
---@param query string
---@param headers? table
function TestFunctionTgql.query(query, headers)
  print("query")
end

--- Check comment
---@param resp table
function TestFunctionTgql.check(resp)
  print("check")
end

---@class TestFunctionTsql
local TestFunctionTsql = {}

--- Query for multiple rows
---@param query string
---@param ... string|boolean|number
function TestFunctionTsql.query(query, ...)
  print("query")
end

--- Query for a single row. Will error if no rows returned
---@param query string
---@param ... string|boolean|number
function TestFunctionTsql.queryRow(query, ...)
  print("queryRow")
end

--- Check comment
---@param resp table
function TestFunctionTsql.check(resp)
  print("check")
end

---@class TestFunctionTpubsub
local TestFunctionTpubsub = {}

--- Check comment
---@param topic string
---@param resp table
function TestFunctionTpubsub.check(topic, resp)
  print("check")
end

---@class TestFunctionTk8s
local TestFunctionTk8s = {}

--- Check if a resource exists in a cluster
---@param apiVersion string
---@param kind string
---@param cluster string
---@param namespace string
---@param name string
---@param resp table
function TestFunctionTk8s.check(apiVersion, kind, cluster, namespace, name, resp)
  print("check")
end

--- Test case
---@class Test
---@field gql fun(name: string, fn: fun(t: TestFunctionTgql))
---@field sql fun(name: string, fn: fun(t: TestFunctionTsql))
---@field pubsub fun(name: string, fn: fun(t: TestFunctionTpubsub))
---@field k8s fun(name: string, fn: fun(t: TestFunctionTk8s))
Test = {}

--- Helper functions
---@class Helper
Helper = {}

--- Execute some SQL. Will error if the SQL fails
---@param query string
---@param ... string|boolean|number
function Helper.SQLExec(query, ...)
  print("SQLExec")
end

--- Execute some SQL. Will return multiple rows.
---@param query string
---@param ... string|boolean|number
---@return table
function Helper.SQLQuery(query, ...)
  print("SQLQuery")
  return {}
end

--- Execute some SQL. Returns a single row. Error if no rows returned
---@param query string
---@param ... string|boolean|number
---@return table
function Helper.SQLQueryRow(query, ...)
  print("SQLQueryRow")
  return {}
end

--- Read in k8s resources from a directory
---@param dir string
function Helper.readK8sResources(dir)
  print("readK8sResources")
end

--- Configuration
---@class Config
---@field SkipSeed boolean
---@field Unauthenticated boolean
---@field Admin boolean
---@field TenantName string
Config = {
  SkipSeed = false,
  Unauthenticated = false,
  Admin = false,
  TenantName = "some-tenant",
}
