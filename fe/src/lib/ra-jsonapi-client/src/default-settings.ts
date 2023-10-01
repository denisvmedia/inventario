export default {
  total: 'total',
  headers: {
    Accept: 'application/vnd.api+json',
    'Content-Type': 'application/vnd.api+json',
  },
  updateMethod: 'PATCH',
  arrayFormat: 'brackets',
  getManyKey: 'id',
} as {
  total: string;
  headers: {
    Accept: string;
    'Content-Type': string;
  };
  updateMethod: string;
  arrayFormat: 'brackets' | 'indices' | 'repeat' | 'comma' | undefined;
  getManyKey: string;
};
