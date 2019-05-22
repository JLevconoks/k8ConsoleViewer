This is a small console app designed for monitoring pod statuses in multiple namespaces. 

How to use:
- Download latest release from [releases page](https://github.com/JLevconoks/k8ConsoleViewer/releases)
- Create `groups.json` file alongside your download in the format similar to `groups-sample.json` 

- Run `./k8ConsoleViewer <id>` or `./k8ConsoleViewer <name>` based on the groups.json
- Run `./k8ConsoleViewer` to view available groups 


Symlink to /url/local/bin to launch it from anywhere 
```
ln -s <path to the app executable> /usr/local/bin/<prefered name>
``` 
for example `ln -s ~/Tools/k8ConsoleViewer/k8ConsoleViewer /usr/local/bin/k8viewer`