import axios from 'axios';
import { HttpError } from 'react-admin';

interface InnerErrors {
  [key: string]: string|Array<any>;
}

interface ParsedError {
  body: {
    message: string;
    status: string;
    errors: InnerErrors;
  };
}

function extractValidationErrors(obj: any): InnerErrors | null {
  if (!Array.isArray(obj?.errors) || !obj?.errors.length) {
    return null;
  }

  const errors = obj!.errors![0]; // TODO: support multiple errors

  if (
    errors?.error?.type === 'validation.Errors'
    && typeof errors?.error?.error?.data?.attributes === 'object'
  ) {
    return errors?.error?.error?.data?.attributes;
  }

  return null;
}

function parseError(input: any): ParsedError | null {
  const errs = extractValidationErrors(input);

  if (!errs) {
    return null;
  }

  // Convert the errors (capitalize the first letter of each message)
  const errors: { [key: string]: string|Array<string> } = {};
  Object.keys(errs).forEach((key) => {
    // check if errs[key] is an object
    if (typeof errs[key] === 'object') {
      errors[key] = [];

      // for each key of object
      Object.keys(errs[key]).forEach((itemKey: any) => {
        // try parse itemKey as integer, return if not a number
        const index = parseInt(itemKey, 10);
        if (Number.isNaN(index)) {
          return;
        }

        (errors[key] as string[])[index] = errs[key][itemKey].charAt(0).toUpperCase() + errs[key][itemKey].slice(1);
      });
    } else if (typeof errs[key] === 'string') {
      errors[key] = (errs[key] as string).charAt(0).toUpperCase() + errs[key].slice(1);
    }
  });

  const status = input?.errors[0]?.status || 'Unknown error';
  const message = input?.errors[0]?.error.type || status;

  // Return the parsed structure
  return {
    body: {
      message,
      status,
      errors,
    },
  };
}

// Handle HTTP errors.
export default () => {
  // Request interceptor
  axios.interceptors.request.use(
    (config) => {
      const token = localStorage.getItem('token');
      const username = localStorage.getItem('username');
      const password = localStorage.getItem('password');

      const newConfig = config;

      // When a 'token' is available set as Bearer token.
      if (token) {
        newConfig.headers.Authorization = `Bearer ${token}`;
      }

      // When username and password are available use
      // as basic auth credentials.
      if (username && password) {
        newConfig.auth = { username, password };
      }

      return newConfig;
    },
    (err) => Promise.reject(err),
  );

  // Response interceptor
  axios.interceptors.response.use(
    (response) => response,
    (error) => {
      const { status, data } = error.response;

      if (status < 200 || status >= 300) {
        const err = parseError(data);
        return Promise.reject(
          new HttpError(
            err?.body.message || 'Unknown error',
            err?.body.status || 'Unknown error',
            err?.body || {},
          ),
        );
      }

      return Promise.reject(error);
    },
  );
};
