export function imgFallback(node, params) {
    window.attachImgFallback(node, params.kind, params.id, params.size);
    if (node.complete && node.naturalWidth === 0 && node.onerror) {
        node.onerror();
    }
    return {
        update(next) {
            window.attachImgFallback(node, next.kind, next.id, next.size);
        }
    };
}
