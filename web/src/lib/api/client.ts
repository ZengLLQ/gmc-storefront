import { ApiError, type ErrorEnvelopePayload } from "./errors";

const SERVER_API_URL =
  process.env.API_INTERNAL_URL || "http://localhost:8080";

let refreshPromise: Promise<boolean> | null = null;

async function attemptSessionRefresh(isAdminPath: boolean): Promise<boolean> {
  if (typeof window === "undefined") {
    return false;
  }
  if (refreshPromise) {
    return refreshPromise;
  }
  const endpoint = isAdminPath
    ? "/api/v1/admin/auth/refresh"
    : "/api/v1/auth/refresh";

  refreshPromise = (async () => {
    try {
      const res = await fetch(endpoint, {
        method: "POST",
        credentials: "include",
        headers: {
          "X-Requested-With": "gmc-web",
        },
      });
      return res.ok;
    } catch {
      return false;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

export interface ApiRequestOptions extends Omit<RequestInit, "body"> {
  body?: unknown;
  params?: Record<string, string | number | boolean | undefined | null>;
  idempotencyKey?: string;
  skipAuthRefresh?: boolean;
  tags?: string[];
  revalidate?: number | false;
}

function buildUrl(
  path: string,
  params?: Record<string, string | number | boolean | undefined | null>
): string {
  const isServer = typeof window === "undefined";
  const cleanPath = path.startsWith("/") ? path : `/${path}`;
  const base = isServer ? `${SERVER_API_URL}${cleanPath}` : `/api${cleanPath}`;

  if (!params) return base;
  const search = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && String(v) !== "") {
      search.set(k, String(v));
    }
  }
  const qs = search.toString();
  return qs ? `${base}?${qs}` : base;
}

export async function apiFetch<T>(
  path: string,
  options: ApiRequestOptions = {}
): Promise<T> {
  const {
    body,
    params,
    idempotencyKey,
    skipAuthRefresh,
    tags,
    revalidate,
    headers: customHeaders,
    ...rest
  } = options;

  const url = buildUrl(path, params);
  const method = (rest.method || "GET").toUpperCase();

  const headers = new Headers(customHeaders);
  headers.set("Accept", "application/json");

  if (method !== "GET" && method !== "HEAD") {
    headers.set("X-Requested-With", "gmc-web");
  }
  if (idempotencyKey) {
    headers.set("Idempotency-Key", idempotencyKey);
  }

  let serializedBody: BodyInit | undefined;
  if (body instanceof FormData) {
    serializedBody = body;
  } else if (body !== undefined) {
    headers.set("Content-Type", "application/json");
    serializedBody = JSON.stringify(body);
  }

  const nextConfig: { tags?: string[]; revalidate?: number | false } = {};
  if (tags) nextConfig.tags = tags;
  if (revalidate !== undefined) nextConfig.revalidate = revalidate;

  const doRequest = () =>
    fetch(url, {
      ...rest,
      method,
      headers,
      body: serializedBody,
      credentials: "include",
      ...(Object.keys(nextConfig).length > 0 ? { next: nextConfig } : {}),
    });

  let res = await doRequest();

  if (
    res.status === 401 &&
    !skipAuthRefresh &&
    typeof window !== "undefined" &&
    !path.includes("/auth/login") &&
    !path.includes("/auth/refresh")
  ) {
    const isAdmin =
      path.startsWith("/v1/admin") ||
      window.location.pathname.startsWith("/admin");
    const refreshed = await attemptSessionRefresh(isAdmin);
    if (refreshed) {
      res = await doRequest();
    }
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const contentType = res.headers.get("content-type") || "";
  if (!res.ok) {
    if (contentType.includes("application/json")) {
      const payload = (await res
        .json()
        .catch(() => ({}))) as ErrorEnvelopePayload;
      throw new ApiError({
        status: res.status,
        code: payload.error?.code,
        message: payload.error?.message,
        fields: payload.error?.fields,
        requestId:
          payload.error?.requestId || res.headers.get("x-request-id") || undefined,
      });
    }
    throw new ApiError({
      status: res.status,
      message: `Permintaan gagal (${res.status})`,
    });
  }

  if (contentType.includes("application/json")) {
    return (await res.json()) as T;
  }
  return (await res.text()) as unknown as T;
}

export const apiGet = <T>(
  path: string,
  params?: Record<string, string | number | boolean | undefined | null>,
  opts?: Omit<ApiRequestOptions, "method" | "params">
) => apiFetch<T>(path, { ...opts, method: "GET", params });

export const apiPost = <T>(
  path: string,
  body?: unknown,
  opts?: Omit<ApiRequestOptions, "method" | "body">
) => apiFetch<T>(path, { ...opts, method: "POST", body });

export const apiPut = <T>(
  path: string,
  body?: unknown,
  opts?: Omit<ApiRequestOptions, "method" | "body">
) => apiFetch<T>(path, { ...opts, method: "PUT", body });

export const apiPatch = <T>(
  path: string,
  body?: unknown,
  opts?: Omit<ApiRequestOptions, "method" | "body">
) => apiFetch<T>(path, { ...opts, method: "PATCH", body });

export const apiDelete = <T>(
  path: string,
  opts?: Omit<ApiRequestOptions, "method">
) => apiFetch<T>(path, { ...opts, method: "DELETE" });
