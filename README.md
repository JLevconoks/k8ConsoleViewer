This is a small console app designed for monitoring pod statuses in multiple namespaces.   
  
At this moment it is aimed at MacOS and iTerm2.  

---
  
## How to use:  
**Please note, each `context/namespace` pair is a separate `get pods` call to Kubernetes with your credentials every 5 seconds, so be considerate with the number of namespaces you are monitoring.**   

- Download latest release from [releases page](https://github.com/JLevconoks/k8ConsoleViewer/releases)  
- Run `./k8ConsoleViewer -c <context> -n <namespace>`   

#### Alternatively 
- Create `groups.json` file alongside your download in the format similar to `groups-sample.json` - Run `./k8ConsoleViewer group <id>` or `./k8ConsoleViewer group <name>` based on the groups.json  
- Run `./k8ConsoleViewer group` to view available groups   
  
Namespace name can contain wildcards for example 'foo*bar' will be converted to regex `^foo.*bar$` and compared to all namespaces in given context. Regex itself is not available, for now.  
  
**When using wildcard namespace name need to be in quotes, to correctly pass parameter to the application.**  
`./k8ConsoleViewer -c foo -n "bar*"`  
  
#### Shortcuts/Hotkeys:  
- `1-9` - copy commands to clipboard, more info in app footer  
- `e` - expand all namespaces  
- `c` - collapse all elements  
- `left` - collapse item / navigate to parent item  
- `right` - expand item  
- `PgUp` - scroll up a page  
- `PgDn` - scroll down a page  
- `Home` - scroll to the top  
- `End` - scroll to the end  
  
---

## iTerm2 integration 
iTerm2 Python Api is used to open new window, split it and execute a command with broadcast.   
  
There are several prerequisites to enable iTerm2 Api:  
- Python3 and dependencies:   
```commandline  
brew install python3  
pip3 install iterm2  
pip3 install pyobjc  
```  
- Enable Python API in iTerm2  
```text Preferences -> General -> Magic -> Enable Python Api ```  
  
#### Shortcuts  
- `Ctrl + E` - exec in all containers in the pod group  
- `Ctrl + L` - get logs from all containers in pod group   
- `Ctrl + K` - follow logs from all containers in pod group   
  
 ---
### Symlink to `/url/local/bin`  
You can make a symlink to `/url/local/bin` to launch it from anywhere `ln -s <path to the app executable> /usr/local/bin/<prefered name>` 

For example `ln -s ~/Tools/k8ConsoleViewer/k8ConsoleViewer /usr/local/bin/k8viewer`  

---
### Updating the app

- Run `./k8ConsoleViewer update` and follow the instructions.   
This will get latest release(if different), backup existing app and replace it with a new version.   
  
(There is an hourly rate limit per IP for the Github releases endpoint, so if you will get 403, try again later)