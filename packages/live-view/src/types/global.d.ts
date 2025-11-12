/// <reference types="jest" />

declare global {
  var global: typeof globalThis;
  var fetch: typeof globalThis.fetch;
  var ResizeObserver: typeof globalThis.ResizeObserver;
  var IntersectionObserver: typeof globalThis.IntersectionObserver;
  var WebSocket: typeof globalThis.WebSocket;
  var RTCPeerConnection: typeof globalThis.RTCPeerConnection;
  var MediaStream: typeof globalThis.MediaStream;
  var MediaStreamTrack: typeof globalThis.MediaStreamTrack;
  var VideoDecoder: typeof globalThis.VideoDecoder;
  var EncodedVideoChunk: typeof globalThis.EncodedVideoChunk;
  var MediaSource: typeof globalThis.MediaSource;
  var URL: typeof globalThis.URL;
}

export {};
