/**
 * Compresses payload object into a gzipped ArrayBuffer for HTTP requests
 * and returns the request options.
 */
export async function compressRequestBody(
  bodyData: any,
): Promise<{ body: ArrayBuffer; headers: Record<string, string> }> {
  const jsonStr = JSON.stringify(bodyData);
  const uint8 = new TextEncoder().encode(jsonStr);

  // Create CompressionStream for gzip compression
  // CompressionStream is natively supported in Safari, Chrome, Firefox, etc.
  const stream = new Response(uint8).body?.pipeThrough(new CompressionStream("gzip"));
  if (!stream) {
    throw new Error("Failed to create compression stream");
  }
  const compressedBuffer = await new Response(stream).arrayBuffer();

  return {
    body: compressedBuffer,
    headers: {
      "Content-Encoding": "gzip",
      "Content-Type": "application/json",
    },
  };
}
