Sonar Project Overall Status Exporter
=====================================

Tested on Sonar 8.9

http://localhost:9000/projects?search=ds-3&sort=duplications

![screenshot](./screenshot.png)

```bash
$ ./sonar-exp -host http://localhost:9000 -t xxxxx -q ds-3 > ds3.csv
$ cat ds3.csv
Project,Bugs,Vulnerabilities,Hotspots Reviewed,Code Smells,Coverage,Duplications,Lines,NCLOC Language Distribution,Size,Duplications*Lines,Bug/Lines*1k%,Code Smells/Lines*1k%
ds-305-master,10,4,0.0,3396,0.0,20.0,60080,java=59046;xml=1034,M,12016.000000,0.166445,56.524635
ds-317-dev,78,11,0.0,4256,0.0,35.7,98510,java=97850;xml=660,M,35168.070312,0.791798,43.203735
```

| Project       | Bugs | Vulnerabilities | Hotspots Reviewed | Code Smells | Coverage | Duplications | Lines | NCLOC Language Distribution | Size | Duplications*Lines | Bug/Lines*1k% | Code Smells/Lines*1k% |
|:--------------|:-----|:----------------|:------------------|:------------|:---------|:-------------|:------|:----------------------------|:-----|:-------------------|:--------------|:----------------------|
| ds-305-master | 10   | 4               | 0.0               | 3396        | 0.0      | 20.0         | 60080 | java=59046;xml=1034         | M    | 12016.000000       | 0.166445      | 56.524635             |
| ds-317-dev    | 78   | 11              | 0.0               | 4256        | 0.0      | 35.7         | 98510 | java=97850;xml=660          | M    | 35168.070312       | 0.791798      | 43.203735             |