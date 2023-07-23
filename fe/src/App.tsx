import {
  Admin,
  Resource,
  EditGuesser,
  ShowGuesser,
} from 'react-admin';
import React from 'react';
import dataProvider from './dataProvider';
import AreaCreate from './components/AreaCreate';
import LocationCreate from './components/LocationCreate';
import LocationList from './components/LocationList';
import AreaList from './components/AreaList';
import LocationEdit from './components/LocationEdit';

function App() {
  return (
    <Admin dataProvider={dataProvider}>
      <Resource
        name="locations"
        list={LocationList}
        create={LocationCreate}
        edit={LocationEdit}
        show={ShowGuesser}
        recordRepresentation="name"
      />
      <Resource
        name="areas"
        list={AreaList}
        create={AreaCreate}
        edit={EditGuesser}
        show={ShowGuesser}
        recordRepresentation="name"
      />
    </Admin>
  );
}

export default App;
