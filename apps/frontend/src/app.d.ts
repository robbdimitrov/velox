declare global {
  namespace App {
    interface Locals {
      user: {
        id: string;
        email: string;
        role: 'customer' | 'vendor' | 'admin';
        name?: string;
      } | null;
    }
    interface PageData {
      gatewayBaseURL?: string;
      user?: App.Locals['user'];
    }
  }
}

export {};
