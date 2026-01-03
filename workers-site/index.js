import { getAssetFromKV } from "@cloudflare/kv-asset-handler";
import manifestJSON from "__STATIC_CONTENT_MANIFEST";

const assetManifest = JSON.parse(manifestJSON);

export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url);

        // Strip /docs prefix for asset lookup
        if (url.pathname.startsWith("/docs")) {
            url.pathname = url.pathname.replace(/^\/docs/, "") || "/";
        }

        const modifiedRequest = new Request(url, request);

        try {
            return await getAssetFromKV(
                { request: modifiedRequest, waitUntil: ctx.waitUntil.bind(ctx) },
                {
                    ASSET_NAMESPACE: env.__STATIC_CONTENT,
                    ASSET_MANIFEST: assetManifest,
                },
            );
        } catch (e) {
            // SPA fallback: serve index.html for docsify hash routing
            url.pathname = "/index.html";
            return await getAssetFromKV(
                {
                    request: new Request(url, request),
                    waitUntil: ctx.waitUntil.bind(ctx),
                },
                {
                    ASSET_NAMESPACE: env.__STATIC_CONTENT,
                    ASSET_MANIFEST: assetManifest,
                },
            );
        }
    },
};
