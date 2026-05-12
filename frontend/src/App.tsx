import { useCallback, useEffect, useState } from "react";
import { fetchList } from "./api";
import type { DirEntry, FileEntry } from "./types";

const PREVIEWABLE = new Set([
  "pdf",
  "md",
  "jpg", "jpeg", "png", "gif", "webp", "svg",
  "mp4", "webm",
  "mp3", "wav", "flac", "ogg",
  "txt", "log", "csv",
  "json", "yaml", "yml", "toml",
  "xml", "html", "htm",
  "js", "ts", "jsx", "tsx",
  "py", "go", "rb", "rs", "java", "c", "cpp", "h", "hpp",
  "sh", "bash", "zsh",
  "css", "scss", "less",
  "sql", "ini", "conf", "env",
]);

const FILE_ICONS: Record<string, string> = {
  pdf: "📄",
  doc: "📝", docx: "📝", txt: "📃", md: "📝",
  jpg: "🖼️", jpeg: "🖼️", png: "🖼️", gif: "🖼️", webp: "🖼️",
  mp4: "🎬", avi: "🎬", mkv: "🎬", mov: "🎬",
  mp3: "🎵", wav: "🎵", flac: "🎵",
  zip: "📦", rar: "📦", "7z": "📦", tar: "📦", gz: "📦",
  js: "💻", ts: "💻", html: "🌐", css: "🎨", py: "🐍", go: "🔵",
  json: "📋", xml: "📋", yaml: "📋", yml: "📋",
  exe: "⚙️", dmg: "💿", iso: "💿",
};

function getIcon(name: string, isDir: boolean): string {
  if (isDir) return "📁";
  const ext = name.split(".").pop()?.toLowerCase();
  return FILE_ICONS[ext ?? ""] ?? "📎";
}

function formatSize(raw: string): string {
  return raw;
}

function encodePath(parts: string[]): string {
  return parts.map(encodeURIComponent).join("/");
}

export default function App() {
  const path = window.location.pathname.replace(/\/$/, "") || "";
  const [dirs, setDirs] = useState<DirEntry[]>([]);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchList(path || "/")
      .then((res) => {
        setDirs(res.dirs);
        setFiles(res.files);
      })
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : "unknown error");
      })
      .finally(() => setLoading(false));
  }, [path]);

  const filteredDirs = query
    ? dirs.filter((d) => d.name.toLowerCase().includes(query.toLowerCase()))
    : dirs;
  const filteredFiles = query
    ? files.filter((f) => f.name.toLowerCase().includes(query.toLowerCase()))
    : files;

  const navigate = useCallback((p: string) => {
    window.location.href = p;
  }, []);

  const segments = path ? path.split("/").filter(Boolean) : [];

  return (
    <div className="min-h-screen bg-gradient-to-br from-[#a8e6cf] via-[#7fcdbb] to-[#41b6c4] p-10 max-sm:p-5">
      <div className="mx-auto max-w-[900px]">
        <header className="mb-10 text-center">
          <h1 className="text-4xl font-semibold text-white drop-shadow-[0_2px_10px_rgba(0,0,0,0.2)] max-sm:text-3xl">
            📁 文件分享
          </h1>
          <p className="mt-2.5 text-white/80 text-lg">快速访问和下载文件</p>
        </header>

        <div className="overflow-hidden rounded-2xl bg-white/95 shadow-[0_20px_60px_rgba(0,0,0,0.3)] backdrop-blur-sm">
          {/* Breadcrumb */}
          <div className="border-b border-gray-200 bg-gray-50/80 px-7 py-3.5 text-sm text-gray-500">
            <a href="/" className="rounded-md px-2 py-1 text-[#41b6c4] no-underline transition-colors hover:bg-[#e6f7f5]">
              📁 根目录
            </a>
            {segments.length > 0 && (
              <span className="mx-1 text-gray-300">/</span>
            )}
            {segments.map((seg, i) => {
              const href = "/" + encodePath(segments.slice(0, i + 1));
              if (i === segments.length - 1) {
                return (
                  <span key={i} className="ml-1 font-medium text-gray-700">
                    {seg}
                  </span>
                );
              }
              return (
                <span key={i}>
                  <a
                    href={href}
                    className="ml-1 rounded-md px-2 py-1 text-[#41b6c4] no-underline transition-colors hover:bg-[#e6f7f5]"
                  >
                    {seg}
                  </a>
                  <span className="ml-1 text-gray-300">/</span>
                </span>
              );
            })}
          </div>

          {/* Search */}
          <div className="flex items-center gap-4 border-b border-gray-200 bg-gray-50/80 px-7 py-5">
            <div className="relative flex-1">
              <span className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 select-none text-base">
                🔍
              </span>
              <input
                type="text"
                placeholder="搜索文件..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="w-full rounded-xl border-2 border-gray-200 bg-white py-3 pl-11 pr-5 text-[15px] outline-none transition-colors focus:border-[#41b6c4]"
              />
            </div>
          </div>

          {/* Content */}
          <div className="p-2.5">
            {loading && (
              <div className="py-20 text-center text-gray-400 text-lg">
                加载中...
              </div>
            )}
            {error && (
              <div className="py-20 text-center">
                <div className="mb-2 text-5xl opacity-50">⚠️</div>
                <h3 className="mb-2 text-xl text-gray-700">加载失败</h3>
                <p className="text-gray-400">{error}</p>
              </div>
            )}
            {!loading && !error && filteredDirs.length === 0 && filteredFiles.length === 0 && (
              <div className="py-20 text-center text-gray-400">
                <div className="mb-2 text-5xl opacity-50">📂</div>
                <h3 className="mb-2 text-xl text-gray-700">暂无内容</h3>
                <p>该目录下还没有任何文件或文件夹</p>
              </div>
            )}
            {!loading && !error && (
              <>
                {filteredDirs.map((d) => (
                  <a
                    key={"d:" + d.name}
                    href={"/" + encodePath([...segments, d.name])}
                    className="mx-2 my-1.5 flex cursor-pointer items-center rounded-xl px-5 py-4 no-underline text-inherit transition-all hover:translate-x-1 hover:bg-[#e6f7f5]"
                  >
                    <div className="mr-4 flex size-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-[#7fcdbb] to-[#41b6c4] text-2xl text-white">
                      {getIcon(d.name, true)}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="break-all text-base font-semibold text-gray-700">
                        {d.name}
                      </div>
                      <div className="mt-1 text-xs text-gray-400">
                        修改时间: {d.time}
                      </div>
                    </div>
                    <div className="shrink-0 rounded-full bg-[#e6f7f5] px-3 py-1.5 text-sm font-medium text-[#2a9d8f]">
                      文件夹
                    </div>
                  </a>
                ))}
                {filteredFiles.map((f) => {
                  const href = "/" + encodePath([...segments, f.name]);
                  const ext = f.name.split(".").pop()?.toLowerCase() ?? "";
                  const isPreviewable = PREVIEWABLE.has(ext);
                  if (isPreviewable) {
                    return (
                      <div
                        key={"f:" + f.name}
                        className="mx-2 my-1.5 flex items-center rounded-xl px-5 py-4 transition-all hover:bg-[#e6f7f5]"
                      >
                        <div className="mr-4 flex size-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-[#7fcdbb] to-[#41b6c4] text-2xl text-white">
                          {getIcon(f.name, false)}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="break-all text-base font-semibold text-gray-700">
                            {f.name}
                          </div>
                          <div className="mt-1 text-xs text-gray-400">
                            修改时间: {f.time}
                          </div>
                        </div>
                        <div className="flex shrink-0 items-center gap-2">
                          <span className="shrink-0 rounded-full bg-[#e6f7f5] px-3 py-1.5 text-sm font-medium text-[#2a9d8f]">
                            {f.size}
                          </span>
                          <a
                            href={href + "?preview=1"}
                            target="_blank"
                            className="inline-block whitespace-nowrap rounded-full border border-[#bee3f8] bg-[#ebf4ff] px-3.5 py-1.5 text-xs font-medium text-[#2b6cb0] no-underline transition-colors hover:bg-[#2b6cb0] hover:text-white"
                          >
                            预览
                          </a>
                          <a
                            href={href}
                            download
                            className="inline-block whitespace-nowrap rounded-full border border-[#b2e0da] bg-[#e6f7f5] px-3.5 py-1.5 text-xs font-medium text-[#2a9d8f] no-underline transition-colors hover:bg-[#2a9d8f] hover:text-white"
                          >
                            下载
                          </a>
                        </div>
                      </div>
                    );
                  }
                  return (
                    <a
                      key={"f:" + f.name}
                      href={href}
                      download
                      className="mx-2 my-1.5 flex cursor-pointer items-center rounded-xl px-5 py-4 no-underline text-inherit transition-all hover:translate-x-1 hover:bg-[#e6f7f5]"
                    >
                      <div className="mr-4 flex size-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-[#7fcdbb] to-[#41b6c4] text-2xl text-white">
                        {getIcon(f.name, false)}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="break-all text-base font-semibold text-gray-700">
                          {f.name}
                        </div>
                        <div className="mt-1 text-xs text-gray-400">
                          修改时间: {f.time}
                        </div>
                      </div>
                      <span className="shrink-0 rounded-full bg-[#e6f7f5] px-3 py-1.5 text-sm font-medium text-[#2a9d8f]">
                        {f.size}
                      </span>
                    </a>
                  );
                })}
              </>
            )}
          </div>

          {/* Footer */}
          <div className="border-t border-gray-200 bg-gray-50/80 px-5 py-4 text-center text-xs text-gray-400">
            <span>
              {dirs.length + files.length} 个项目 ({dirs.length} 个文件夹,{" "}
              {files.length} 个文件)
            </span>
            &nbsp;·&nbsp;
            <a
              href="https://github.com/loveuer/ufshare"
              target="_blank"
              className="text-gray-400 no-underline hover:text-gray-600"
            >
              github.com/loveuer/ufshare
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
