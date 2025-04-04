NETCONF client for nokia 1830 PSS equipment.

## Usage
`./gonc -ip 10.10.10.10 -password admin -username admin -port 830 -file payloads/otdr.xml -output output.xml -filter "/rpc-reply/data/terminal-device/logical-channels/channel[start-with(index,'10115')]"`

- `-filter is a feature to add the unsupported start-with filter to the result. It does the filtering by doing post-processing on the response from NE`
---
### What is NETCONF?

NETCONF is a network management protocol designed to configure and manage network devices (routers, switches, firewalls, etc.) in a standardized, programmatic way. Think of it as a more modern, structured alternative to SNMP or CLI scripting. Unlike SNMP, which is great for monitoring but clunky for configuration, or CLI, which is human-friendly but not machine-friendly, NETCONF is built for automation and precision.

It was created by the IETF (RFC 4741, later updated in RFC 6241) to address the chaos of vendor-specific CLI commands and the limitations of SNMP’s configuration capabilities. NETCONF uses XML for data encoding and operates over a secure transport like SSH, making it structured and reliable.

Imagine NETCONF as a REST API for network devices, but with a twist: instead of stateless HTTP requests, it’s session-based, and it uses a rich data model (YANG) to define what you can configure and retrieve. It’s like having a contract between you and the device about what’s possible.

### How Does NETCONF Work?

NETCONF is a client-server protocol:
- **Client**: Your management tool or script.
- **Server**: The network device.

The communication happens in **sessions** over a secure transport (usually SSH on port 830). Within a session, the client sends **RPCs** (Remote Procedure Calls) to the server, and the server responds with XML-formatted data.

Here’s the basic flow:
1. **Establish a session**: The client connects to the device.
2. **Exchange capabilities**: Both sides say, “Here’s what I can do.” (More on this later.)
3. **Send operations**: The client tells the device what to do (e.g., get config, edit config).
4. **Receive replies**: The device responds with success, failure, or data.
5. **Close the session**: Done!

### Key Concepts in NETCONF

Let’s break down the essentials:

#### 1. **Operations**
NETCONF defines specific operations you can perform on a device. These are like API endpoints, but they’re XML-based RPCs. The main ones are:
- `<get>`: Retrieve running state data (think: “show” commands in CLI or a GET in REST).
- `<get-config>`: Fetch configuration data from a specific datastore (more on datastores soon).
- `<edit-config>`: Modify the device’s configuration (like a POST or PUT in REST).
- `<commit>`: Apply changes staged in a candidate configuration (if supported).
- `<lock>` / `<unlock>`: Prevent others from editing the config while you’re working.
- `<copy-config>`: Copy one configuration to another (e.g., replace running config with a file).
- `<delete-config>`: Wipe a configuration datastore.

- Example: To fetch a device’s running config, you’d send:
    ```xml
    <rpc message-id="101" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
      <get-config>
        <source>
          <running/>
        </source>
      </get-config>
    </rpc>
    ```
The device replies with XML containing the config.

#### 2. **Datastores**
Think of datastores as “databases” of configuration and state data on the device. NETCONF separates these clearly:
- **Running**: The active configuration currently running on the device (like SNMP’s live data or CLI’s “show running-config”).
- **Candidate**: A scratchpad for staging changes before applying them (optional, not all devices support it).
- **Startup**: The config loaded when the device boots (like “show startup-config” in CLI).

When you `<edit-config>`, you specify which datastore to target (e.g., `<candidate>` or `<running>`). This is a big leap from SNMP, which doesn’t distinguish these cleanly, or CLI, where it’s vendor-specific.

#### 3. **Capabilities**
When a NETCONF session starts, the client and server exchange a `<hello>` message listing their capabilities. This is like a handshake saying, “I support these features; what about you?” Examples:
- `:base:1.1`: Supports NETCONF 1.1 features.
- `:candidate`: Supports the candidate datastore.
- `:xpath`: Allows filtering data with XPath expressions.

This ensures you only try operations the device can handle, avoiding the guesswork of CLI scripting.

#### 4. **XML Encoding**
All NETCONF messages are XML. If you’ve used REST with JSON, think of XML as a more verbose but equally structured format. It’s machine-readable and hierarchical, perfect for complex configs. Example response:
```xml
<rpc-reply message-id="101" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <data>
    <interface>
      <name>eth0</name>
      <ip>192.168.1.1</ip>
    </interface>
  </data>
</rpc-reply>
```
### Enter YANG Models

Think of **YANG** (Yet Another Next Generation) is a data modeling language (RFC 7950) that defines the structure, syntax, and semantics of the data you can manage via NETCONF.

#### What is a YANG Model?
A YANG model is a blueprint for what a device can do. It describes:
- **Configuration data**: What you can set (e.g., IP addresses, VLANs).
- **State data**: What you can read (e.g., interface status, counters).
- **RPCs**: Custom operations the device supports.
- **Notifications**: Events the device can send (e.g., “interface down”).

Here’s a simple YANG example:
```yang
module example-interfaces {
  namespace "urn:example:interfaces";
  prefix "if";

  container interfaces {
    list interface {
      key "name";
      leaf name {
        type string;
      }
      leaf ip {
        type string;
      }
    }
  }
}
```
This defines a structure with a list of interfaces, each with a `name` and `ip`. It’s like a JSON schema but for NETCONF.

#### How YANG Ties to NETCONF
- The device exposes its YANG models as part of its capabilities.
- Your NETCONF client uses these models to craft valid `<get>` or `<edit-config>` requests.
- The XML payload you send must match the YANG structure, or the device rejects it.

For example, to set an IP address using the model above:
```xml
<rpc message-id="102" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <edit-config>
    <target>
      <running/>
    </target>
    <config>
      <interfaces xmlns="urn:example:interfaces">
        <interface>
          <name>eth0</name>
          <ip>192.168.1.1</ip>
        </interface>
      </interfaces>
    </config>
  </edit-config>
</rpc>
```

#### Types of YANG Models
- **Standard Models**: Defined by organizations like IETF (e.g., `ietf-interfaces` for interface config).
- **Vendor Models**: Nokia, Cisco, Juniper ., extend standard models with proprietary features.
- **Custom Models**: You can write your own for specific use cases.

### NETCONF vs. What You Know
- **vs. CLI**: CLI is ad-hoc and vendor-specific. NETCONF is standardized, structured, and scriptable.
- **vs. SNMP**: SNMP is polling-based and weak at configuration. NETCONF is session-based, configuration-first, and uses YANG for clarity.
- **vs. REST API**: REST is stateless and often JSON-based. NETCONF is session-based, XML-based, and tied to YANG models.

### Putting It Together: A Workflow
1. **Connect**: Start a NETCONF session (e.g., via SSH to port 830).
2. **Check Capabilities**: Read the device’s `<hello>` to see its YANG models and features.
3. **Get Data**: Use `<get-config>` to fetch the running config.
4. **Edit Config**: Use `<edit-config>` with XML based on the YANG model to make changes.
5. **Commit**: If using a candidate datastore, `<commit>` to apply changes.
6. **Validate**: Use `<get>` to confirm the state.

### Next Steps
To get hands-on:
- **Devices**: Check if your network gear supports NETCONF.
- **YANG**: Explore public YANG repos like [YANG GitHub](https://github.com/YangModels/yang) to see real models.