import React, { useState } from 'react';

const [error] = useState('');

function xx() {
  if (!error) {
    return null;
  }

  if (error.type === 'validation.Errors' && error.error?.data) {
    if (typeof e.error?.data === 'object' && Object.keys(e.error.data).length > 0) {

    }
  }

  return <span>x</span>;
}

function Errors() {
  return (
    <div>
      <span>Errors</span>
      {error && <div>{error}</div>}
    </div>
  );
}

export default Errors();
