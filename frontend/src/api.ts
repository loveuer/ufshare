import type { ListResponse } from "./types";

export async function fetchList(path: string): Promise<ListResponse> {
  const params = new URLSearchParams({ path });
  const res = await fetch(`/api/list?${params}`);
  if (!res.ok) {
    throw new Error(`failed to list: ${res.status}`);
  }
  return res.json();
}
