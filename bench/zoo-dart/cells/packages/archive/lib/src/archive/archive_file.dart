// the shape of archive 4.x: the name and the target of a link are plain fields
class ArchiveFile {
  String name;
  int size;
  String? symbolicLink;
  String? comment;

  ArchiveFile(this.name, this.size, List<int> data);
  ArchiveFile.symlink(this.name, String this.symbolicLink) : size = 0;
}
