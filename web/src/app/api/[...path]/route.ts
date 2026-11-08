import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const API_INTERNAL_URL =
  process.env.API_INTERNAL_URL || "http://localhost:8080";
const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || "http://localhost:3000";

async function proxyHandler(
  req: NextRequest,
  context: { params: Promise<{ path: string[] }> }
): Promise<NextResponse> {
  const { path } = await context.params;
  const targetPath = "/" + (path || []).join("/");
  const search = req.nextUrl.search || "";
  const upstreamUrl = `${API_INTERNAL_URL}${targetPath}${search}`;

  const upstreamHeaders = new Headers();
  const forwardHeaderNames = [
    "accept",
    "content-type",
    "cookie",
    "authorization",
    "idempotency-key",
    "if-none-match",
    "user-agent",
    "x-request-id",
  ];

  for (const name of forwardHeaderNames) {
    const val = req.headers.get(name);
    if (val) {
      upstreamHeaders.set(name, val);
    }
  }

  // Ensure CSRF & Origin headers are forwarded for mutating requests
  upstreamHeaders.set(
    "x-requested-with",
    req.headers.get("x-requested-with") || "gmc-web"
  );
  upstreamHeaders.set("origin", req.headers.get("origin") || SITE_URL);

  const clientIp =
    req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ||
    req.headers.get("x-real-ip") ||
    "127.0.0.1";
  upstreamHeaders.set("x-forwarded-for", clientIp);

  let body: BodyInit | undefined;
  if (req.method !== "GET" && req.method !== "HEAD") {
    const buf = await req.arrayBuffer();
    if (buf.byteLength > 0) {
      body = buf;
    }
  }

  try {
    const upstreamRes = await fetch(upstreamUrl, {
      method: req.method,
      headers: upstreamHeaders,
      body,
      redirect: "manual",
      cache: "no-store",
    });

    const responseHeaders = new Headers();
    const copyResHeaders = [
      "content-type",
      "cache-control",
      "etag",
      "x-request-id",
      "retry-after",
      "content-disposition",
    ];
    for (const h of copyResHeaders) {
      const val = upstreamRes.headers.get(h);
      if (val) {
        responseHeaders.set(h, val);
      }
    }

    // Preserve all Set-Cookie headers accurately
    const setCookies =
      typeof upstreamRes.headers.getSetCookie === "function"
        ? upstreamRes.headers.getSetCookie()
        : [];
    if (setCookies.length > 0) {
      for (const cookie of setCookies) {
        responseHeaders.append("Set-Cookie", cookie);
      }
    } else {
      const rawCookie = upstreamRes.headers.get("set-cookie");
      if (rawCookie) {
        responseHeaders.append("Set-Cookie", rawCookie);
      }
    }

    if (upstreamRes.status === 204 || upstreamRes.status === 304) {
      return new NextResponse(null, {
        status: upstreamRes.status,
        headers: responseHeaders,
      });
    }

    return new NextResponse(upstreamRes.body, {
      status: upstreamRes.status,
      headers: responseHeaders,
    });
  } catch {
    return NextResponse.json(
      {
        error: {
          code: "internal",
          message: "Tidak dapat terhubung ke layanan backend GMC.",
          requestId: req.headers.get("x-request-id") || "proxy-err",
        },
      },
      { status: 502 }
    );
  }
}

export const GET = proxyHandler;
export const POST = proxyHandler;
export const PUT = proxyHandler;
export const PATCH = proxyHandler;
export const DELETE = proxyHandler;
export const OPTIONS = proxyHandler;
