Files to JSON Lines
===================

将一个路径下的多个文本文件（可按文件类型过滤）内容，输出成一个 [JSON Lines](https://jsonlines.org/) 格式文件。
输出的文件中，每行表示一个输入文件的 JSON 字符串。

具体格式如下：

```json lines
{"text": "content_of_source_file_1", "url": "absolute_path_to_source_file_1"}
{"text": "content_of_source_file_2", "url": "absolute_path_to_source_file_2"}
{"text": "content_of_source_file_3", "url": "absolute_path_to_source_file_3"}
...
```

用法示例：

```bash
./files2jsonl -d /path/to/src \
-i xml,pom,java,groovy,yml,yaml,properties,json,sql,htm,vue,json,html,js,md,sh \
-o /path/to/target
```

- `-d` 指定源文件路径
- `-i` 指定需要包含的文件类型，不区分大小写，可省略，表示包含全部文件
- `-o` 指定输出文件路径，默认输出到当前路径