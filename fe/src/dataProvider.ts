// import fakeRestDataProvider from 'ra-data-fakerest';
// import jsonServerProvider from 'ra-data-json-server';
import jsonapiClient from 'ra-jsonapi-client';
// import data from './data.json';

// const dataProvider = fakeRestDataProvider(data, true);
const dataProvider = jsonapiClient('http://localhost:3333/api/v1',
  {
    total: null,
    updateMethod: 'PUT',
  },
);

export default dataProvider;
