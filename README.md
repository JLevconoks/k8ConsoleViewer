This is a small console app designed for monitoring pod statuses in multiple namespaces. 

How to use:
- Download latest release from [releases page](https://github.com/JLevconoks/k8ConsoleViewer/releases)
- Create `groups.json` file alongside your download in the format below: 
```json
[
  {
    "id": 0,
    "name": "jenkins",
    "context": "tools",
    "namespaces": [
      "jenkins-slaves"
    ]
  },
  {
      "id": 1,
      "name": "mynamespace",
      "context": "dev",
      "namespaces": [
        "app1_namespace",
        "app2_namespace"
      ]
  }
]
```
- Run `./k8ConsoleViewer <id>` or `./k8ConsoleViewer <name>` based on the groups.json (Initial launch might be a bit slow due to kubectl response times)
- Run `./k8ConsoleViewer` to view available groups 
