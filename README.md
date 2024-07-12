# Summary

A service for searching datasets, tools, collections etc. using ElasticSearch.

# Prerequisites

Access to an ElasticSearch deployment, with URL, username and password information.

# Endpoints

```
POST /settings/datasets
```
Defines the index settings for `datasets` in ElasticSearch.
This process only needs to be run once when initially setting up the index. 
No body required. 

```
POST /settings/tools
```
Defines the index settings for `tools` in ElasticSearch.
This process only needs to be run once when initially setting up the index. 
No body required.    

```
GET /search
{
    "query": "asthma icd10"
}
```
This is the endpoint to perform a search.
It searches over the elastic indices of the available entity types (datasets, tools and collections) for the given query term.
Results are returned grouped by entity type.

## Example search results structure

```
{
    "datasets": {
        "took": 169,
        "timed_out": false,
        "_shards": {...},
        "hits": {
            "hits": [
                "_explanation": {...},
                "id": "1",
                "index": "datasets",
                "node": "aaaa1111",
                "score": 7.3,
                "shard": "[datasets][0]",
                "source": {...},
                "highlight": {...}
            ]
            "max_score": 7.3,
            "total": {}
        }
    }
}
```

## Logging

To enable the audit log locally, the user needs to define the environment variables below and have a copy of `application_default_credentials.json` copied into the root directory of the container.

```
AUDIT_LOG_ENABLED="true"
PUBSUB_PROJECT_ID=
PUBSUB_TOPIC_NAME=
PUBSUB_SERVICE_NAME="search-service"
GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials_file/in/container"
```

To enable debug level console logging set the environment variable `DEBUG_LOGGING="true"`.
