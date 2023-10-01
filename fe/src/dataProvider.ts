import { LegacyDataProvider } from 'react-admin';

// eslint-disable-next-line import/no-relative-packages
import jsonapiClient from './lib/ra-jsonapi-client/src';

// const dataProvider = fakeRestDataProvider(data, true);
const dataProvider : LegacyDataProvider = jsonapiClient(
  'http://localhost:3333/api/v1',
  {
    total: null,
    updateMethod: 'PUT',
  },
);

export default dataProvider;
