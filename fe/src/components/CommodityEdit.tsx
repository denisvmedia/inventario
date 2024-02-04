import {
  useRecordContext,
  Edit,
} from 'react-admin';
import React from 'react';
import ComodityBaseForm from './ComodityBaseForm';

function CommodityEdit() {
  return (
    <Edit mutationMode="pessimistic">
      {ComodityBaseForm()}
    </Edit>
  );
}

export default CommodityEdit;
