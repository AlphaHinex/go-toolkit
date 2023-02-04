Upload Jars
===========

简介
---

批量上传 Jar 包至 Maven 仓库的命令行工具。
如果存在与 Jar 包同名的 pom 文件，也会一并上传。

Jar 包及 pom 文件可都放置在同一路径内，并将此路径通过 `-i` 参数传入。

若仅有 Jar 包没有 pom 文件，需将 Jar 包按照要上传至的位置，放置在三级路径内：`GruopId/ArtifactId/Version`

示例
----

例如将 Jar 包及 pom 文件按如下路径放置：

```
├── activiti-spring-5.15.jar
├── activiti-spring-5.15.pom
├── c3p0
│   └── c3p0
│       └── 0.9
│           └── c3p0-0.9-SNAPSHOT.jar
├── c3p0-0.9.1.2.jar
├── upload-jars
```

在相同路径执行 `./upload-jars -s snapshoturl -r release-url` 后，相当于执行了如下 Maven 命令：

```bash
$ mvn deploy:deploy-file \
-DgroupId=c3p0 -DartifactId=c3p0 -Dversion=0.9 \
-Dpackaging=jar -Dfile=./c3p0/c3p0/0.9/c3p0-0.9-SNAPSHOT.jar -Durl=snapshot-url
$ mvn deploy:deploy-file \
-DgroupId=org.activiti -DartifactId=activiti-spring -Dversion=5.15 \
-Dpackaging=jar -Durl=release-url \
-DpomFile=./activiti-spring-5.15.pom -Dfile=./activiti-spring-5.15.jar
```

> 因 `c3p0-0.9.1.2.jar` 无同名 pom 文件，无法确定其 GAV 信息，无法被上传。

更多内容可见帮助信息：`./upload-jars -h`