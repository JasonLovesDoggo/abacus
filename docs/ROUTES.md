
# /hit/<namespace>/<key>

#### Function:
This route is used to increment and fetch the value of a key in a namespace.

- **GET**: Get the value of a key in a namespace.
  - **namespace**: The namespace of the key.
  - **key**: The key to get the value of.
  - **Response**: The value of the key in the namespace.
  - **Status Code**: 200 if the key exists, 404 if the key does not exist.
  - **Example**: `GET /hit/visitors/total` returns `1000`. and would increment the key by 1.


# /get/<namespace>/<key> 
                                    
#### Function:
This route is used to fetch the value of a key in a namespace without incrementing.
- **GET**: Get the value of a key in a namespace.
  - **namespace**: The namespace of the key.
  - **key**: The key to get the value of.
  - **Response**: The value of the key in the namespace.
  - **Status Code**: 200 if the key exists, 404 if the key does not exist.
  - **Example**: `GET /get/visitors/total` returns `1000`.
