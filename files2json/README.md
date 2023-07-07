Files to JSON
=============

将文本文件内容，输出成一个 JSON 字符串。可输入多个文件，输出的文件中，每行表示一个输入文件的 JSON 字符串。

具体格式如下：

```json
{"text": "content_of_source_file_1", "url": "absolute_path_to_source_file_1"}
{"text": "content_of_source_file_2", "url": "absolute_path_to_source_file_2"}
{"text": "content_of_source_file_3", "url": "absolute_path_to_source_file_3"}
...
```

用法示例：

```bash
./files2json -d /path/to/src -i xml,java,yml,properties,json
```

- `-d` 指定源文件路径
- `-i` 指定文件类型