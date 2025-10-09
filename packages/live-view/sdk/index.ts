import React from 'react';
import ReactDOM from 'react-dom/client';
import { AndroidLiveviewComponent, AndroidLiveviewComponentProps } from "./AndroidLiveviewComponent";


export function createAndroidLiveView(mount: HTMLElement, props: AndroidLiveviewComponentProps) {
  const root = (ReactDOM as any).createRoot(mount);
  root.render(React.createElement(AndroidLiveviewComponent, props));
  return root;
}
