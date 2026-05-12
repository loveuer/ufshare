export interface DirEntry {
  name: string;
  time: string;
}

export interface FileEntry {
  name: string;
  size: string;
  time: string;
}

export interface ListResponse {
  path: string;
  dirs: DirEntry[];
  files: FileEntry[];
}
