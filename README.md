# lsweep-tools
Lansweeper scanning tools (Agent Proxy, Mock Sender, Mock Receiver)

## Todo
- [x] Add communication documentation w/ examples
- [x] Add mock receiver
- [x] Add mock sender
- [ ] Add agent proxy

## Background
We have a couple of assets that are separated network-wise from our Lansweeper server which we wanted to scan, but since the cloud relay is US-based that wasn't an option for us. This led us to a dive into the communication between agent and server, which is described below.

## The tools
### Agent Proxy (lsweep-agent-proxy/)
Simple relay that sits between the Lansweeper agent and the server. Will generally only forward intercepted traffic with one major exception - when the agent sends a config request the last part of the response from Lansweeper will be altered to have the traffic go through the proxy instead of trying to call the server directly.

The agent proxy is writen in go, and currently has no SSL support so will need to be run behind a reverse proxy (e.g. nginx, haproxy) that can handle SSL termination.

### Mock Sender (mocksweeper/agent.py)
Simple tool that can send mock agent reports to the LANSweeper server for fictional machines. Mainly there to illustrate the communication, has not been tested for production use.

### Mock Receiver (mocksweeper/server.py)
In case you want to test the communication from agent to server, this tool impersonates the agent data receiver (normally) on port 9524. Mainly to illustrate the communication, has not been tested for production use.

#### Usage
```bash
# Install prerequisites
pip3 install -r requirements.txt

# Run the mock server
# The hostname is used to tell the client which host to connect to
python3 server.py hostname.internal.com 8011

# Server will listen on 0.0.0.0 by default
```


## General communication info
### Request base
All requests are made via POST to /lsagent with the content type multipart/form-data (with one exception, see #2)

### Request headers
The following headers are sent with all requests:

| Header | Default | Description |
| --- | --- | --- |
| User-Agent | None | The useragent for the request from the agent, seems to be missing in most requests made between agent and server but will not impact communication |
| Content-Type | multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz | Content type for the request, including boundary information for body |
| Content-Type | application/x-www-form-urlencoded | For some requests, the content type is set to application/x-www-form-urlencoded even though the body format is multipart/form-data. |
| Content-Length | None | The length of the request body, normally generated automatically |

### Request body
The body is always in the multipart/form-data format, separated with the boundary from the headers.

| Postfield | Default | Description |
| --- | --- | --- |
| Action | Hello | The action that the request performs, you can find the actions in the chain below |

## Request-Response chain

### 1. Action: Hello
#### POST Fields
| Postfield | Default | Description |
| --- | --- | --- |
| Action | Hello | Sent by the agent when installed/run, confirms connectivity to the server |
#### Request
##### Headers
```
POST /lsagent HTTP/2

Host: lansweeper.example.com
Content-Type: multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Length: 190
```
##### Body
```
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="Action"
Content-Type: application/octet-stream

Hello
------------abcdefghijklmnopqrstuvwxyz--
```

#### Response
```html
HTTP/2 200 OK
Server: Microsoft-HTTPAPI/2.0

OK
```

### 2. Action: AssetStatus
#### POST fields
| Postfield | Default | Description |
| --- | --- | --- |
| Action | AssetStatus | Retrieve the current status of the asset defined by the AssetId |
| AssetId | 01234567-89ab-cdef-0123-456789abcde | Asset Identifier, generated locally by agent on installation |
| OperatingSystem | Linux | The operating system where the agent is installed |

#### Request
##### Headers
NOTE: When sent from the official agent, Content-Type is set to application/x-www-form-urlencoded even though the body is formatted according to multipart/form-data.
```
POST /lsagent HTTP/2

Host: lansweeper.example.com
Content-Type: multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Length: 521
```

##### Body
```
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="AssetId"
Content-Type: application/octet-stream

01234567-89ab-cdef-0123-456789abcde
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="OperatingSystem"
Content-Type: application/octet-stream

Linux
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="Action"
Content-Type: application/octet-stream

AssetStatus
------------abcdefghijklmnopqrstuvwxyz--
```

#### Response
Response seems to be the same whether the Asset ID exists or not, might differ based on if an object is disabled or has any other status.

##### Headers
```
HTTP/2 200 OK

Content-Type: multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Length: 123
Server: Microsoft-HTTPAPI/2.0

```

##### Body
```
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="Status"
Content-Type: application/octet-stream

Enabled
------------abcdefghijklmnopqrstuvwxyz--
```

### 3. Action: Config
#### POST Fields
| Field | Default | Description |
| --- | --- | --- |
| Action | Config | Get the scanning configuration from the LANSweeper server (frequency, HTTP endpoint, etc.) |
| AssetId | 01234567-89ab-cdef-0123-456789abcde | The Asset Identifier |
| OperatingSystem | Linux | The operating system where the agent is installed |

#### Request
##### Headers
```
POST /lsagent HTTP/2

Host: lansweeper.example.com
Content-Type: multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Length: 521
```

##### Body
```
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="AssetId"
Content-Type: application/octet-stream

01234567-89ab-cdef-0123-456789abcde
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="OperatingSystem"
Content-Type: application/octet-stream

Linux
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="Action"
Content-Type: application/octet-stream

Config
------------abcdefghijklmnopqrstuvwxyz--
```

#### Response
##### Headers
```
HTTP/2 200 OK

Content-Type: multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Length: 123
Server: Microsoft-HTTPAPI/2.0
```

##### Body
```
--multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="Config"; filename="Config"
Content-Type: application/octet-stream

[[ CONFIGURATION XML (examples/scan-configuration.xml)]]

--multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz--
```


### 4. Action: ScanData
#### Scan data compression

The Scan field is a gzipped JSON string joined with a number packed into a bytestring representing the length of the original Json data before compression. Examples below:

```python
fakepc = {
    "attr": "value",
    "attr2": [
        "value1",
        "value2
    ]
}

# Convert to oneline json string and encode
json_bytes = json.dumps(fakepc, separators=(',', ':')).encode('utf-8')

# Gzip the json string
compressed = gzip.compress(json_bytes)

# Get original json_bytes length and att packed number at end of data
compressed += struct.pack('<I', len(json_bytes))
```

```csharp
using System.IO.Compression;
using System.Text;

string text = "{\"attr\": \"value\", \"attr2\": [\"value1\", \"value2\"]}"
byte[] bArray = Encoding.UTF8.GetBytes(text);

MemoryStream memStream = new MemoryStream();
using (GZipStream gzipStream = new GZipStream((Stream) memStream, CompressionMode.Compress, true))
gzipStream.Write(buffer, 0, buffer.Length);

memStream.Position = 0L;
byte[] numArray = new byte[memStream.Length];
memStream.Read(numArray, 0, numArray.Length);

byte[] dst = new byte[numArray.Length + 4];
Buffer.BlockCopy((Array) numArray, 0, (Array) dst, 0, numArray.Length);
Buffer.BlockCopy((Array) BitConverter.GetBytes(bArray.Length), 0, (Array) dst, numArray.Length, 4);
```

#### POST Fields
| Field | Default | Description |
| --- | --- | --- |
| Action | ScanData | Send updated configuration to Lansweeper server |
| AssetId | 01234567-89ab-cdef-0123-456789abcde | The asset identifier |
| OperatingSystem | Linux | The operating system where the agent is installed |
| Scan (filename="Scan") | Gzipped Configuration XML | The gzipped XML containing the scan information |

#### Data
- examples/scan-result.json: The json file with information about the server
- examples/scan-result-min.json: Minified before compression
- examples/scan-result-min-gzipped.json.gz: Minified and gzipped
- examples/scan-result-min-gzipped-with-length-bytes.json.gz: Added length bytes according to Scan data compression above

#### Request
##### Headers
```
POST /lsagent HTTP/2

Host: lansweeper.example.com
Content-Type: multipart/form-data; boundary=----------abcdefghijklmnopqrstuvwxyz
Content-Length: 36960
```

##### Body
```
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="AssetId"
Content-Type: application/octet-stream

01234567-89ab-cdef-0123-456789abcde
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="OperatingSystem"
Content-Type: application/octet-stream

Linux
------------abcdefghijklmnopqrstuvwxyz
Content-Disposition: form-data; name="Action"
Content-Type: application/octet-stream

ScanData

------------08aae60b7f464026967e486314dea904
Content-Disposition: form-data; name="Scan"; filename="Scan"
Content-Type: application/octet-stream

[Scan Data, Gzipped Json + packed integer representing original json length (examples/scan-result-min-gzipped-with-length-bytes.json.gz)]
------------abcdefghijklmnopqrstuvwxyz--
```

#### Response
```
HTTP/2 200 OK

Server: Microsoft-HTTPAPI/2.0

OK
```