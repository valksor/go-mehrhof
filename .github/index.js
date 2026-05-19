import { getAssetFromKV } from '@cloudflare/kv-asset-handler'
import manifestJSON from '__STATIC_CONTENT_MANIFEST'

const assetManifest = JSON.parse(manifestJSON)

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url)

    // Redirect directory paths without trailing slash (fixes relative path resolution)
    if (
      url.pathname.match(/^\/docs\/kvelmo(\/|$)/) &&
      !url.pathname.endsWith('/') &&
      !url.pathname.split('/').pop().includes('.')
    ) {
      return Response.redirect(url.origin + url.pathname + '/', 301)
    }

    // Strip /docs/kvelmo/latest or /docs/kvelmo/nightly prefix for asset lookup
    if (url.pathname.startsWith('/docs/kvelmo/latest')) {
      url.pathname = url.pathname.replace(/^\/docs\/kvelmo\/latest/, '') || '/'
    } else if (url.pathname.startsWith('/docs/kvelmo/nightly')) {
      url.pathname = url.pathname.replace(/^\/docs\/kvelmo\/nightly/, '') || '/'
    }

    const modifiedRequest = new Request(url, request)

    try {
      return await getAssetFromKV(
        { request: modifiedRequest, waitUntil: ctx.waitUntil.bind(ctx) },
        {
          ASSET_NAMESPACE: env.__STATIC_CONTENT,
          ASSET_MANIFEST: assetManifest,
        },
      )
    } catch (e) {
      // SPA fallback: serve index.html for docsify hash routing
      url.pathname = '/index.html'
      return await getAssetFromKV(
        {
          request: new Request(url, request),
          waitUntil: ctx.waitUntil.bind(ctx),
        },
        {
          ASSET_NAMESPACE: env.__STATIC_CONTENT,
          ASSET_MANIFEST: assetManifest,
        },
      )
    }
  },
}
