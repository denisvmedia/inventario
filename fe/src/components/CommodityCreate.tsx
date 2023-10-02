import {
  Create,
} from 'react-admin';
import React from 'react';
import ComodityBaseForm from './ComodityBaseForm';

function CommodityCreate() {
  return (
    <Create>
      {ComodityBaseForm()}
    </Create>
  );
}

export default CommodityCreate;
