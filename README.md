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
